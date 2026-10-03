package authn

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestIssuer is an issuer the tests control: its JWKS served by an httptest server, tokens signed here.
func TestVerify(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	edPub, edKey, _ := ed25519.GenerateKey(rand.Reader)
	var fetches atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"keys": []JWK{RSAJWK("r1", &rsaKey.PublicKey), ECJWK("e1", &ecKey.PublicKey), EdJWK("d1", edPub)}})
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0)
	access := Issuer{Kind: "access", Issuer: "https://team.cloudflareaccess.com", JWKS: srv.URL, Audience: "aud-tag"}
	oidc := Issuer{Kind: "oidc", Issuer: "https://auth.test", JWKS: srv.URL, Audience: "https://api.test"}
	v := &Verifier{Client: srv.Client(), Now: func() time.Time { return now }}
	claims := func(change func(map[string]any)) map[string]any {
		c := map[string]any{"iss": access.Issuer, "aud": []string{"aud-tag"}, "exp": now.Unix() + 60, "iat": now.Unix(), "nbf": now.Unix(), "type": "app"}
		if change != nil {
			change(c)
		}
		return c
	}

	for name, c := range map[string]struct {
		token string
		want  Identity
		err   error
	}{
		"an Access person":        {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["email"] = "dev@example.com"; c["sub"] = "u1" })), Identity{Kind: "access", Email: "dev@example.com", Subject: "u1"}, nil},
		"an Access service token": {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["common_name"] = "abc.access"; c["sub"] = "" })), Identity{Kind: "access", ServiceToken: "abc.access"}, nil},
		"an OIDC user, RS256, scopes": {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) {
			c["iss"], c["aud"], c["sub"], c["scope"] = oidc.Issuer, oidc.Audience, "user-7", "openid devices:read"
		})), Identity{Kind: "oidc", Subject: "user-7"}, nil},
		"ES256, which is not taken": {Sign("ES256", "e1", ecKey, claims(func(c map[string]any) { c["iss"], c["aud"], c["sub"] = oidc.Issuer, oidc.Audience, "user-9" })), Identity{}, ErrSignature},
		"an OIDC user, EdDSA":       {Sign("EdDSA", "d1", edKey, claims(func(c map[string]any) { c["iss"], c["aud"], c["sub"] = oidc.Issuer, oidc.Audience, "user-8" })), Identity{Kind: "oidc", Subject: "user-8"}, nil},
		"an untrusted issuer":       {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["iss"], c["email"] = "https://evil.test", "x@y" })), Identity{}, ErrIssuer},
		"another audience":          {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["aud"], c["email"] = "other", "x@y" })), Identity{}, ErrClaims},
		"expired":                   {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["exp"], c["email"] = now.Unix()-120, "x@y" })), Identity{}, ErrClaims},
		"not yet valid":             {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["nbf"], c["email"] = now.Unix()+120, "x@y" })), Identity{}, ErrClaims},
		"no exp":                    {Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { delete(c, "exp"); c["email"] = "x@y" })), Identity{}, ErrClaims},
		"no one":                    {Sign("RS256", "r1", rsaKey, claims(nil)), Identity{}, ErrClaims},
		"alg none":                  {Sign("none", "r1", rsaKey, claims(func(c map[string]any) { c["email"] = "x@y" })), Identity{}, ErrSignature},
		"an RSA key as ES256":       {resign(Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["email"] = "x@y" })), "ES256"), Identity{}, ErrSignature},
		"a changed claim":           {tamper(Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) { c["email"] = "x@y" }))), Identity{}, ErrSignature},
		"an unknown key":            {Sign("RS256", "nope", rsaKey, claims(func(c map[string]any) { c["email"] = "x@y" })), Identity{}, ErrSignature},
		"not a JWT":                 {"abc.def", Identity{}, ErrMalformed},
	} {
		got, err := v.Verify(context.Background(), c.token, access, oidc)
		if !errors.Is(err, c.err) {
			t.Errorf("%s: error %v, want %v", name, err, c.err)
			continue
		}
		if err == nil && (got.Kind != c.want.Kind || got.Email != c.want.Email || got.ServiceToken != c.want.ServiceToken || got.Subject != c.want.Subject) {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
		}
	}
	// Keys are fetched once, then again only for an unknown kid and not more than every 30 s.
	if n := fetches.Load(); n != 1 {
		t.Errorf("the keys were fetched %d times, want 1", n)
	}
	got, _ := v.Verify(context.Background(), Sign("RS256", "r1", rsaKey, claims(func(c map[string]any) {
		c["iss"], c["aud"], c["sub"], c["scope"] = oidc.Issuer, oidc.Audience, "u", "a b"
	})), oidc)
	if len(got.Scopes) != 2 || got.Scopes[1] != "b" {
		t.Errorf("scopes: %v", got.Scopes)
	}
}

func TestAccessIssuer(t *testing.T) {
	for _, team := range []string{"team.cloudflareaccess.com", "https://team.cloudflareaccess.com/"} {
		if got := AccessIssuer(team, "a"); got.Issuer != "https://team.cloudflareaccess.com" || got.JWKS != "https://team.cloudflareaccess.com/cdn-cgi/access/certs" {
			t.Errorf("%s: %+v", team, got)
		}
	}
}

func tamper(token string) string {
	b := []byte(token)
	i := len(b) / 2
	if b[i] == 'A' {
		b[i] = 'B'
	} else {
		b[i] = 'A'
	}
	return string(b)
}

func resign(token, alg string) string {
	var parts [3]string
	copy(parts[:], split(token))
	header, _ := json.Marshal(map[string]string{"alg": alg, "kid": "r1"})
	return base64.RawURLEncoding.EncodeToString(header) + "." + parts[1] + "." + parts[2]
}

func split(token string) []string {
	var out []string
	start := 0
	for i := range token {
		if token[i] == '.' {
			out = append(out, token[start:i])
			start = i + 1
		}
	}
	return append(out, token[start:])
}

// Sign makes a JWT; a test issuer's half of Verify. alg "none" leaves the signature empty.
func Sign(alg, kid string, key crypto.Signer, claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": alg, "kid": kid, "typ": "JWT"})
	body, _ := json.Marshal(claims)
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	var signature []byte
	sum := sha256.Sum256([]byte(signed))
	switch k := key.(type) {
	case *rsa.PrivateKey:
		if alg != "none" {
			signature, _ = rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
		}
	case *ecdsa.PrivateKey:
		r, s, _ := ecdsa.Sign(rand.Reader, k, sum[:])
		signature = append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	case ed25519.PrivateKey:
		signature = ed25519.Sign(k, []byte(signed))
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func RSAJWK(kid string, k *rsa.PublicKey) JWK {
	return JWK{Kty: "RSA", Kid: kid, N: base64.RawURLEncoding.EncodeToString(k.N.Bytes()), E: base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})}
}

func ECJWK(kid string, k *ecdsa.PublicKey) JWK {
	return JWK{Kty: "EC", Crv: "P-256", Kid: kid, X: base64.RawURLEncoding.EncodeToString(k.X.FillBytes(make([]byte, 32))), Y: base64.RawURLEncoding.EncodeToString(k.Y.FillBytes(make([]byte, 32)))}
}

func EdJWK(kid string, k ed25519.PublicKey) JWK {
	return JWK{Kty: "OKP", Crv: "Ed25519", Kid: kid, X: base64.RawURLEncoding.EncodeToString(k)}
}
