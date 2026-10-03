// Package authntest is an issuer for tests: it serves its keys over HTTP, as Cloudflare Access
// (/cdn-cgi/access/certs) and as an OpenID Connect provider (/jwks), and signs tokens with them.
// What a test trusts it with is only ever a test's own configuration.
package authntest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/joeblew999/fleet-api/authn"
)

// Issuer is a running test issuer.
type Issuer struct {
	URL    string // its origin: the Access team domain, and the OIDC issuer
	Server *httptest.Server
	key    *rsa.PrivateKey
}

// New starts an issuer; Close it after.
func New() *Issuer {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	i := &Issuer{key: key}
	i.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(i.JWKS())
	}))
	i.URL = i.Server.URL
	return i
}

func (i *Issuer) Close() { i.Server.Close() }

// JWKS is the issuer's key set document.
func (i *Issuer) JWKS() []byte {
	b, _ := json.Marshal(map[string]any{"keys": []authn.JWK{{
		Kty: "RSA", Kid: "test",
		N: base64.RawURLEncoding.EncodeToString(i.key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
	}}})
	return b
}

// Access is what Access would sign for a person (email) or a service token (clientID), for the
// application aud.
func (i *Issuer) Access(aud, email, clientID string) string {
	claims := map[string]any{"aud": []string{aud}, "type": "app"}
	if email != "" {
		claims["email"], claims["sub"] = email, "user-"+email
	} else {
		claims["common_name"], claims["sub"] = clientID, ""
	}
	return i.Token(claims)
}

// OIDC is an access token for audience with these scopes (space-separated).
func (i *Issuer) OIDC(audience, subject, scope string) string {
	return i.Token(map[string]any{"aud": audience, "sub": subject, "scope": scope})
}

// Token signs claims (RS256), adding iss, iat and a 5-minute exp unless given.
func (i *Issuer) Token(claims map[string]any) string {
	now := time.Now().Unix()
	for name, value := range map[string]any{"iss": i.URL, "iat": now, "exp": now + 300} {
		if _, given := claims[name]; !given {
			claims[name] = value
		}
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	signed := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, i.key, crypto.SHA256, sum[:])
	if err != nil {
		panic(err)
	}
	return signed + "." + base64.RawURLEncoding.EncodeToString(signature)
}
