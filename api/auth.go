package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/fleet-api/authn"
)

// Who may call what is declared once, in the contract: each operation's Security names the
// schemes a caller can use and the scope it needs (`requires`). The middleware here enforces
// exactly that; nothing else in the handlers checks a token. docs/concepts/auth.md is the story.

// The security schemes (components.securitySchemes in the spec).
const (
	// AccessClientID and AccessClientSecret are a Cloudflare Access service token, the two
	// headers a machine sends. Access checks them at the edge and hands the Worker a JWT naming the
	// token (Cf-Access-Jwt-Assertion), which is what the Worker verifies. A person who logged in
	// through Access in a browser arrives the same way, with the JWT naming them.
	AccessClientID     = "accessClientId"
	AccessClientSecret = "accessClientSecret"
	// OIDC is an access token from an OpenID Connect provider, in Authorization: Bearer: people and
	// apps. Which provider is the deployment's choice (OIDC_ISSUER); the spec points at
	// /.well-known/openid-configuration on the API, which sends a client on to it.
	OIDC = "oidc"
	// Bearer is the read and write tokens (READ_TOKEN, WRITE_TOKEN), kept for one release while the
	// machines move to Access service tokens.
	Bearer = "bearer"
)

// The scopes. An operation requires one; a caller is granted some (grants).
const (
	ScopeRead   = "devices:read"   // read every machine
	ScopeWrite  = "devices:write"  // post reports: a machine, for itself
	ScopeForget = "devices:forget" // forget a machine: a person
)

// Scopes is every scope, with what it allows: the spec's description of them.
var Scopes = map[string]string{
	ScopeRead:   "Read every machine and its history",
	ScopeWrite:  "Post a machine's report; a machine's own service token posts only its own",
	ScopeForget: "Forget a machine and its reports",
}

// requires is the Security of an operation that needs scope: any of the schemes, with that scope.
// OpenAPI 3.1 lets a requirement on a scheme that is not OAuth list roles; the scope is that role.
func requires(scope string) []map[string][]string {
	return []map[string][]string{
		{AccessClientID: {scope}, AccessClientSecret: {}},
		{OIDC: {scope}},
		{Bearer: {scope}},
	}
}

// securitySchemes is components.securitySchemes.
func securitySchemes() map[string]*huma.SecurityScheme {
	// x-fern-header: the SDKs' option for it, and the variable they read it from when it is not given.
	header := func(name, option, variable, what string) *huma.SecurityScheme {
		return &huma.SecurityScheme{
			Type: "apiKey", In: "header", Name: name, Description: what,
			Extensions: map[string]any{"x-fern-header": map[string]any{"name": option, "env": variable}},
		}
	}
	return map[string]*huma.SecurityScheme{
		AccessClientID:     header("CF-Access-Client-Id", "accessClientId", "FLEET_API_ACCESS_CLIENT_ID", "A Cloudflare Access service token's Client ID: a machine's own (mise run access:token -- create)"),
		AccessClientSecret: header("CF-Access-Client-Secret", "accessClientSecret", "FLEET_API_ACCESS_CLIENT_SECRET", "The service token's Client Secret"),
		OIDC: {
			Type: "openIdConnect", OpenIDConnectURL: "/.well-known/openid-configuration",
			Description: "An access token from the OpenID Connect provider the deployment trusts (OIDC_ISSUER), with the operation's scope",
		},
		Bearer: {Type: "http", Scheme: "bearer", Description: "The read or write token (READ_TOKEN, WRITE_TOKEN): kept for one release, then removed"},
	}
}

// Caller is who a request comes from, once its credentials are verified, and what it may do.
type Caller struct {
	Scheme string         // the scheme that vouched for it: AccessClientID, OIDC or Bearer
	Who    string         // an email, a service token's Client ID, a user's id, or "read token"/"write token"
	Grants []string       // its scopes
	Device string         // a machine's service token: the one device it may post for
	ID     authn.Identity // what the token said, for Access and OIDC
}

func (c Caller) has(scope string) bool {
	for _, g := range c.Grants {
		if g == scope {
			return true
		}
	}
	return false
}

type callerKey struct{}

// CallerOf is the request's verified caller, if it has one.
func CallerOf(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(Caller)
	return c, ok
}

// issuers is what this deployment trusts: Access for its application (ACCESS_TEAM_DOMAIN,
// ACCESS_AUD), an OpenID Connect provider (OIDC_ISSUER, OIDC_JWKS, OIDC_AUDIENCE). Either can be
// unset; unset, it is not trusted.
func (env Env) issuers() (access, oidc *authn.Issuer) {
	if team, aud := env.get("ACCESS_TEAM_DOMAIN"), env.get("ACCESS_AUD"); team != "" && aud != "" {
		i := authn.AccessIssuer(team, aud)
		access = &i
	}
	if iss, jwks, aud := env.get("OIDC_ISSUER"), env.get("OIDC_JWKS"), env.get("OIDC_AUDIENCE"); iss != "" && jwks != "" && aud != "" {
		oidc = &authn.Issuer{Kind: OIDC, Issuer: iss, JWKS: jwks, Audience: aud}
	}
	return access, oidc
}

func (env Env) get(name string) string {
	if env.Var == nil {
		return ""
	}
	return env.Var(name)
}

// identify verifies what the request carries and works out its caller. A request with nothing is
// no caller (ok false, err nil); one whose credentials do not verify is an error.
//
//   - Access's JWT (Cf-Access-Jwt-Assertion), when Access is configured: a person (email) reads; a
//     service token reads, and posts for the one device it was enrolled for (the machines table).
//   - Authorization: Bearer with a JWT, when an OIDC provider is configured: the token's scopes.
//   - Authorization: Bearer with the read or write token.
func (env Env) identify(ctx context.Context, verifier *authn.Verifier, r *http.Request) (Caller, bool, error) {
	access, oidc := env.issuers()
	if jwt := r.Header.Get(authn.AccessHeader); jwt != "" && access != nil {
		id, err := verifier.Verify(ctx, jwt, *access)
		if err != nil {
			return Caller{}, false, err
		}
		// A person who logged in reads and forgets machines; a machine reads, and posts for itself.
		c := Caller{Scheme: AccessClientID, Who: id.Name(), Grants: []string{ScopeRead, ScopeForget}, ID: id}
		if id.Machine() {
			c.Who, c.Grants = id.ServiceToken, []string{ScopeRead}
			store, err := env.Store()
			if err != nil {
				return Caller{}, false, err
			}
			device, enrolled, err := store.Machine(ctx, id.ServiceToken)
			if err != nil {
				return Caller{}, false, err
			}
			if enrolled {
				c.Device, c.Grants = device, []string{ScopeRead, ScopeWrite}
			}
		}
		return c, true, nil
	}
	token := bearer(r.Header.Get("Authorization"))
	if token == "" {
		return Caller{}, false, nil
	}
	if strings.Count(token, ".") == 2 && oidc != nil {
		id, err := verifier.Verify(ctx, token, *oidc)
		if err != nil {
			return Caller{}, false, err
		}
		c := Caller{Scheme: OIDC, Who: id.Name(), ID: id}
		for _, s := range id.Scopes {
			if _, known := Scopes[s]; known {
				c.Grants = append(c.Grants, s)
			}
		}
		return c, true, nil
	}
	switch {
	case env.matches(token, WriteToken):
		return Caller{Scheme: Bearer, Who: "write token", Grants: []string{ScopeRead, ScopeWrite, ScopeForget}}, true, nil
	case env.matches(token, ReadToken):
		return Caller{Scheme: Bearer, Who: "read token", Grants: []string{ScopeRead}}, true, nil
	}
	return Caller{}, false, errUnknownToken
}

type authError string

func (e authError) Error() string { return string(e) }

const errUnknownToken = authError("not a token this API knows")

// authorize is the contract's security, enforced: an operation with an empty Security is open
// (hello); any other needs a caller that one of its requirements lets in, with the scope that
// requirement names. Then the rules no scope can say (allowed).
//
// 401 when there is no caller or its credentials did not verify; 403 when the caller is known but
// may not do this.
func (env Env) authorize(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op := ctx.Operation()
		security := op.Security
		if security == nil {
			// An operation that forgot to say is closed, not open (the contract test fails on it too).
			security = requires(ScopeWrite)
		}
		if len(security) == 0 {
			next(ctx)
			return
		}
		caller, ok := CallerOf(ctx.Context())
		if !ok {
			ctx.SetHeader("WWW-Authenticate", `Bearer resource_metadata="`+contextOrigin(ctx)+`/.well-known/oauth-protected-resource"`)
			message := "credentials are required: a Cloudflare Access service token (CF-Access-Client-Id, CF-Access-Client-Secret), an Access login, or Authorization: Bearer <token>"
			if failed, _ := ctx.Context().Value(failedKey{}).(string); failed != "" {
				message = "the credentials were refused: " + failed
			}
			huma.WriteErr(api, ctx, http.StatusUnauthorized, message)
			return
		}
		var needed []string
		allowed := false
		for _, requirement := range security {
			scopes, applies := requirement[caller.Scheme]
			if !applies {
				continue
			}
			if len(scopes) == 0 || caller.has(scopes[0]) {
				allowed = true
				break
			}
			needed = append(needed, scopes...)
		}
		if !allowed {
			ctx.SetHeader("WWW-Authenticate", `Bearer error="insufficient_scope", scope="`+strings.Join(needed, " ")+`"`)
			huma.WriteErr(api, ctx, http.StatusForbidden, caller.Who+" may not do this: it needs "+strings.Join(needed, " or "))
			return
		}
		if err := rules(caller, op, ctx); err != nil {
			huma.WriteErr(api, ctx, http.StatusForbidden, err.Error())
			return
		}
		next(ctx)
	}
}

// rules is what a scope cannot say: the rules about which object. Today one: a machine's
// service token posts only for the device it was enrolled for. This is where a relation check
// (authz-core: may <caller> <relation> <object>?) goes when there are more.
func rules(caller Caller, op *huma.Operation, ctx huma.Context) error {
	if caller.Device != "" && op.Method != http.MethodGet && ctx.Param("id") != caller.Device {
		return authError(caller.Who + " is this machine's token, for device " + caller.Device + ": it may not post for " + ctx.Param("id"))
	}
	return nil
}

type failedKey struct{}

// withCaller is the request with its caller (or why its credentials failed) in its context, where
// the REST routes and the MCP tool calls (which keep the context) find it.
func (env Env) withCaller(verifier *authn.Verifier, r *http.Request) *http.Request {
	caller, ok, err := env.identify(r.Context(), verifier, r)
	switch {
	case err != nil:
		return r.WithContext(context.WithValue(r.Context(), failedKey{}, err.Error()))
	case ok:
		return r.WithContext(context.WithValue(r.Context(), callerKey{}, caller))
	}
	return r
}

// matches reports whether token is the secret's value, in constant time. An unset secret matches nothing.
func (env Env) matches(token, secret string) bool {
	want := env.get(secret)
	return want != "" && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1
}

func bearer(authorization string) string {
	scheme, token, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// wellKnown answers the two discovery documents a client finds the provider by:
//
//   - /.well-known/oauth-protected-resource (RFC 9728, what MCP clients read): this API, the
//     provider it trusts, its scopes.
//   - /.well-known/openid-configuration, the spec's openIdConnectUrl: a redirect to the provider's.
//
// 404 when no provider is configured.
func (env Env) wellKnown(w http.ResponseWriter, r *http.Request) {
	_, oidc := env.issuers()
	if oidc == nil {
		http.Error(w, "no OpenID Connect provider is configured for this API", http.StatusNotFound)
		return
	}
	if r.URL.Path == "/.well-known/openid-configuration" {
		http.Redirect(w, r, strings.TrimSuffix(oidc.Issuer, "/")+"/.well-known/openid-configuration", http.StatusFound)
		return
	}
	scopes := []string{}
	for s := range Scopes {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"resource": origin(r), "authorization_servers": []string{oidc.Issuer},
		"scopes_supported": scopes, "bearer_methods_supported": []string{"header"},
	})
}

// contextOrigin is origin for a Huma context: the URL is absolute on Workers, Host and TLS say it on net/http.
func contextOrigin(ctx huma.Context) string {
	u := ctx.URL()
	if u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	if ctx.TLS() != nil {
		return "https://" + ctx.Host()
	}
	return "http://" + ctx.Host()
}
