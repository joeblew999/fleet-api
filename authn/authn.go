// Package authn verifies the JSON Web Tokens an API trusts, locally: the signature against the
// issuer's published keys (JWKS, fetched once and kept), then the issuer, the audience and the
// times. It does not decide what a caller may do (that is the API's, from the scopes its contract
// declares); it says who the caller is.
//
// Two kinds of issuer are expected, and any number of each:
//
//   - Cloudflare Access (AccessIssuer): its edge checks a person's login or a service token and
//     sends the Worker a JWT in the Cf-Access-Jwt-Assertion header. RS256, keys at
//     https://<team>.cloudflareaccess.com/cdn-cgi/access/certs.
//   - An OpenID Connect provider such as Better Auth: a client sends its access token in
//     Authorization: Bearer. RS256 or EdDSA (Better Auth's default), keys at the provider's JWKS URL.
//
// Only the standard library, and nothing that needs reflection beyond encoding/json, so it builds
// with TinyGo for Workers (crypto/rsa and crypto/ed25519 verify there). ES256 is left out on
// purpose: crypto/ecdsa adds 176 KB to the gzipped Wasm (TinyGo 0.42, 3 Oct 2026), RSA and Ed25519
// together 113 KB.
package authn

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AccessHeader is the header Cloudflare Access puts its JWT in, on every request it lets through.
const AccessHeader = "Cf-Access-Jwt-Assertion"

// Issuer is one issuer the API trusts.
type Issuer struct {
	// Kind is the API's name for what this issuer vouches for, for example "access" or "oidc":
	// it is copied into Identity.Kind.
	Kind string
	// Issuer is the exact iss claim.
	Issuer string
	// JWKS is the URL of the issuer's public keys.
	JWKS string
	// Audience is a value the aud claim must hold: Access's application AUD tag, or the API's
	// identifier at an OIDC provider.
	Audience string
}

// AccessIssuer is Cloudflare Access for one application: team is the team domain
// (<team>.cloudflareaccess.com, with or without https://), aud the application's AUD tag.
// A team given with http:// keeps it: a test issuer on localhost.
func AccessIssuer(team, aud string) Issuer {
	team = strings.TrimSuffix(team, "/")
	if !strings.HasPrefix(team, "http://") {
		team = "https://" + strings.TrimPrefix(team, "https://")
	}
	return Issuer{Kind: "access", Issuer: team, JWKS: team + "/cdn-cgi/access/certs", Audience: aud}
}

// Identity is who a verified token says the caller is.
type Identity struct {
	Kind    string // the Issuer's Kind
	Issuer  string
	Subject string // sub: a user's id; empty for an Access service token
	// Email is a person's address: Access sets it for a person who logged in, an OIDC provider
	// when the email scope was granted.
	Email string
	// ServiceToken is the Client ID of the Access service token the request came with
	// (CF-Access-Client-Id, the JWT's common_name), for a machine. Empty for a person.
	ServiceToken string
	// Scopes are what an OIDC access token grants (scope, space-separated, or scp). Access gives none.
	Scopes  []string
	Expires time.Time
}

// Machine reports whether the caller is a machine with an Access service token, not a person.
func (id Identity) Machine() bool { return id.ServiceToken != "" }

// Name is the caller as a log line or an error message would name it.
func (id Identity) Name() string {
	switch {
	case id.Email != "":
		return id.Email
	case id.ServiceToken != "":
		return "service token " + id.ServiceToken
	}
	return id.Subject
}

// The errors Verify can return, besides a failure to fetch keys.
var (
	ErrMalformed = errors.New("not a JWT")
	ErrIssuer    = errors.New("an issuer this API does not trust")
	ErrSignature = errors.New("the signature does not verify")
	ErrClaims    = errors.New("the token is expired, not yet valid, or for another audience")
)

// Verifier checks tokens against a set of issuers, keeping each issuer's keys between requests.
// The zero value is not usable: give it a Client (on Workers, workers-go's fetch client).
type Verifier struct {
	Client *http.Client
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Leeway is the clock skew allowed on exp, nbf and iat. Zero is one minute.
	Leeway time.Duration

	mu   sync.Mutex
	sets map[string]*keySet // by JWKS URL
}

type keySet struct {
	keys    map[string]crypto.PublicKey // by kid
	fetched time.Time
}

// How long a fetched key set is kept, and the least time between two fetches of one set (a token
// with an unknown kid makes the Verifier fetch again, at most this often).
const (
	keysKept     = time.Hour
	refetchAfter = 30 * time.Second
)

func (v *Verifier) now() time.Time {
	if v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// Verify checks token against the issuer its iss claim names, which must be one of issuers, and
// returns who it says the caller is.
func (v *Verifier) Verify(ctx context.Context, token string, issuers ...Issuer) (Identity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Identity{}, ErrMalformed
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	var claims struct {
		Iss        string          `json:"iss"`
		Sub        string          `json:"sub"`
		Aud        json.RawMessage `json:"aud"`
		Exp        float64         `json:"exp"`
		Nbf        float64         `json:"nbf"`
		Iat        float64         `json:"iat"`
		Email      string          `json:"email"`
		CommonName string          `json:"common_name"`
		Scope      string          `json:"scope"`
		Scp        []string        `json:"scp"`
	}
	if decodePart(parts[0], &header) != nil || decodePart(parts[1], &claims) != nil {
		return Identity{}, ErrMalformed
	}
	var issuer *Issuer
	for i := range issuers {
		if issuers[i].Issuer != "" && issuers[i].Issuer == claims.Iss {
			issuer = &issuers[i]
			break
		}
	}
	if issuer == nil {
		return Identity{}, ErrIssuer
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Identity{}, ErrMalformed
	}
	key, err := v.key(ctx, issuer.JWKS, header.Kid)
	if err != nil {
		return Identity{}, err
	}
	if !verify(header.Alg, key, []byte(parts[0]+"."+parts[1]), signature) {
		return Identity{}, ErrSignature
	}
	now, leeway := v.now(), v.Leeway
	if leeway == 0 {
		leeway = time.Minute
	}
	at := func(seconds float64) time.Time { return time.Unix(int64(seconds), 0) }
	if claims.Exp == 0 || now.After(at(claims.Exp).Add(leeway)) ||
		(claims.Nbf != 0 && now.Add(leeway).Before(at(claims.Nbf))) ||
		(claims.Iat != 0 && now.Add(leeway).Before(at(claims.Iat))) ||
		!audience(claims.Aud, issuer.Audience) {
		return Identity{}, ErrClaims
	}
	id := Identity{Kind: issuer.Kind, Issuer: claims.Iss, Subject: claims.Sub, Email: claims.Email, Expires: at(claims.Exp)}
	if issuer.Kind == "access" && claims.Email == "" {
		// A service token: Access names it by its Client ID and leaves sub empty.
		id.ServiceToken = claims.CommonName
	}
	if claims.Scope != "" {
		id.Scopes = strings.Fields(claims.Scope)
	} else {
		id.Scopes = claims.Scp
	}
	if id.Email == "" && id.ServiceToken == "" && id.Subject == "" {
		return Identity{}, ErrClaims
	}
	return id, nil
}

func decodePart(part string, into any) error {
	b, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

// audience reports whether aud (a string or a list of them) holds want.
func audience(aud json.RawMessage, want string) bool {
	if want == "" {
		return false
	}
	var one string
	if json.Unmarshal(aud, &one) == nil {
		return one == want
	}
	var many []string
	if json.Unmarshal(aud, &many) == nil {
		for _, a := range many {
			if a == want {
				return true
			}
		}
	}
	return false
}

func verify(alg string, key crypto.PublicKey, signed, signature []byte) bool {
	sum := sha256.Sum256(signed)
	switch k := key.(type) {
	case *rsa.PublicKey:
		return alg == "RS256" && rsa.VerifyPKCS1v15(k, crypto.SHA256, sum[:], signature) == nil
	case ed25519.PublicKey:
		return alg == "EdDSA" && ed25519.Verify(k, signed, signature)
	}
	return false
}

// key is the issuer's key kid, from the kept set, or from a fresh fetch when the set is old or
// does not have it.
func (v *Verifier) key(ctx context.Context, url, kid string) (crypto.PublicKey, error) {
	v.mu.Lock()
	set := v.sets[url]
	v.mu.Unlock()
	now := v.now()
	if set != nil && now.Sub(set.fetched) < keysKept {
		if k, ok := set.keys[kid]; ok {
			return k, nil
		}
		if now.Sub(set.fetched) < refetchAfter {
			return nil, ErrSignature
		}
	}
	keys, err := v.fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	v.mu.Lock()
	if v.sets == nil {
		v.sets = map[string]*keySet{}
	}
	v.sets[url] = &keySet{keys: keys, fetched: now}
	v.mu.Unlock()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, ErrSignature
}

// JWK is one key of a JWKS document: RSA or OKP Ed25519 (an EC key is skipped).
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Crv string `json:"crv,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

func (v *Verifier) fetch(ctx context.Context, url string) (map[string]crypto.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching the keys at %s: %w", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil || res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching the keys at %s: HTTP %d %v", url, res.StatusCode, err)
	}
	return ParseJWKS(body)
}

// ParseJWKS reads a JWKS document ({"keys": [...]}) into its keys by kid, skipping keys of a kind
// it cannot use.
func ParseJWKS(doc []byte) (map[string]crypto.PublicKey, error) {
	var set struct {
		Keys []JWK `json:"keys"`
	}
	if err := json.Unmarshal(doc, &set); err != nil {
		return nil, err
	}
	keys := map[string]crypto.PublicKey{}
	for _, k := range set.Keys {
		if key := k.public(); key != nil {
			keys[k.Kid] = key
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("no usable key in the JWKS")
	}
	return keys, nil
}

func (k JWK) public() crypto.PublicKey {
	b := func(s string) []byte { v, _ := base64.RawURLEncoding.DecodeString(s); return v }
	switch {
	case k.Kty == "RSA" && k.N != "" && k.E != "":
		e := new(big.Int).SetBytes(b(k.E))
		if !e.IsInt64() || e.Int64() < 3 {
			return nil
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(b(k.N)), E: int(e.Int64())}
	case k.Kty == "OKP" && k.Crv == "Ed25519":
		if x := b(k.X); len(x) == ed25519.PublicKeySize {
			return ed25519.PublicKey(x)
		}
	}
	return nil
}
