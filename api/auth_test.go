package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/joeblew999/charter/go/humaworkers"

	"github.com/joeblew999/fleet-api/authn/authntest"
)

const (
	testAUD      = "test-aud"
	testAudience = "https://fleet-api.test"
	machineToken = "machine.access" // a service token's Client ID, enrolled for exampleID
	ciToken      = "ci.access"      // one that was not enrolled
)

// A server that trusts a test issuer as both Cloudflare Access and an OpenID Connect provider, and
// still takes the bearer tokens.
func trusting(t *testing.T) (*httptest.Server, *authntest.Issuer) {
	t.Helper()
	issuer := authntest.New()
	t.Cleanup(issuer.Close)
	memory := &MemStore{}
	memory.Enrol(machineToken, exampleID)
	settings := map[string]string{
		"APP_NAME": "test", ReadToken: testRead, WriteToken: testWrite,
		"ACCESS_TEAM_DOMAIN": issuer.URL, "ACCESS_AUD": testAUD,
		"OIDC_ISSUER": issuer.URL, "OIDC_JWKS": issuer.URL + "/jwks", "OIDC_AUDIENCE": testAudience,
	}
	srv := httptest.NewServer(Handler(Env{Var: func(name string) string { return settings[name] }, Store: func() (Store, error) { return memory, nil }}))
	t.Cleanup(srv.Close)
	return srv, issuer
}

func TestWhoMayDoWhat(t *testing.T) {
	srv, issuer := trusting(t)
	other := "00aa11bb22cc33dd"
	report := func(id string) string { return edit(t, func(m map[string]any) { m["id"] = id }) }
	access := func(token string) []string { return []string{"Cf-Access-Jwt-Assertion", token} }
	bearerOf := func(token string) []string { return []string{"Authorization", "Bearer " + token} }
	person := access(issuer.Access(testAUD, "dev@example.com", ""))
	machine := access(issuer.Access(testAUD, "", machineToken))
	ci := access(issuer.Access(testAUD, "", ciToken))
	for _, c := range []struct {
		name, method, path, body string
		headers                  []string
		status                   int
		says                     string
	}{
		{"a person through Access reads", "GET", "/api/devices", "", person, 200, ""},
		{"a person through Access may not post", "POST", "/api/devices/" + exampleID + "/reports", report(exampleID), person, 403, "devices:write"},
		{"a machine posts for its own device", "POST", "/api/devices/" + exampleID + "/reports", report(exampleID), machine, 201, ""},
		{"a machine may not post for another", "POST", "/api/devices/" + other + "/reports", report(other), machine, 403, "for device " + exampleID},
		{"a machine reads", "GET", "/api/devices/" + exampleID, "", machine, 200, ""},
		{"a machine may not forget itself", "DELETE", "/api/devices/" + exampleID, "", machine, 403, "devices:forget"},
		{"a person through Access forgets a machine", "DELETE", "/api/devices/" + exampleID, "", person, 200, ""},
		{"a service token not enrolled reads", "GET", "/api/devices", "", ci, 200, ""},
		{"a service token not enrolled may not post", "POST", "/api/devices/" + exampleID + "/reports", report(exampleID), ci, 403, "devices:write"},
		{"an Access JWT for another application", "GET", "/api/devices", "", access(issuer.Access("another-app", "dev@example.com", "")), 401, "refused"},
		{"a forged Access JWT", "GET", "/api/devices", "", access(issuer.Access(testAUD, "dev@example.com", "")[:40] + "x.y.z"), 401, "refused"},
		{"an OIDC token with devices:read reads", "GET", "/api/devices", "", bearerOf(issuer.OIDC(testAudience, "user-1", "openid devices:read")), 200, ""},
		{"an OIDC token with devices:read may not post", "POST", "/api/devices/" + exampleID + "/reports", report(exampleID), bearerOf(issuer.OIDC(testAudience, "user-1", "devices:read")), 403, "devices:write"},
		{"an OIDC token with devices:write posts", "POST", "/api/devices/" + other + "/reports", report(other), bearerOf(issuer.OIDC(testAudience, "app-1", "devices:write")), 201, ""},
		{"an OIDC token with no scope of ours", "GET", "/api/devices", "", bearerOf(issuer.OIDC(testAudience, "user-1", "openid email")), 403, "devices:read"},
		{"an OIDC token for another API", "GET", "/api/devices", "", bearerOf(issuer.OIDC("https://elsewhere.test", "user-1", "devices:read")), 401, "refused"},
		{"the write token still posts for any device", "POST", "/api/devices/" + other + "/reports", edit(t, func(m map[string]any) { m["id"], m["ts"] = other, 7 }), bearerOf(testWrite), 201, ""},
		{"nothing", "GET", "/api/devices", "", nil, 401, "credentials are required"},
	} {
		status, answer, _ := do(t, c.method, srv.URL+c.path, c.body, c.headers...)
		if status != c.status || !strings.Contains(answer, c.says) {
			t.Errorf("%s: HTTP %d %s, want %d with %q", c.name, status, answer, c.status, c.says)
		}
	}
	// An MCP tool call runs as the same caller.
	status, body, _ := do(t, "POST", srv.URL+"/api/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"listDevices","arguments":{}}}`,
		append(person, "MCP-Protocol-Version", "2025-06-18")...)
	if status != 200 || strings.Contains(body, `"isError":true`) {
		t.Errorf("MCP as a person: HTTP %d %s", status, body)
	}
}

func TestDiscovery(t *testing.T) {
	srv, issuer := trusting(t)
	status, body, _ := do(t, "GET", srv.URL+"/.well-known/oauth-protected-resource", "")
	var doc struct {
		Resource string   `json:"resource"`
		Servers  []string `json:"authorization_servers"`
		Scopes   []string `json:"scopes_supported"`
	}
	if status != 200 || json.Unmarshal([]byte(body), &doc) != nil || doc.Resource != srv.URL || len(doc.Servers) != 1 || doc.Servers[0] != issuer.URL || len(doc.Scopes) != len(Scopes) {
		t.Errorf("protected resource metadata: HTTP %d %s", status, body)
	}
	// With no provider configured there is nothing to discover.
	plain, _ := server(t)
	if status, _, _ := do(t, "GET", plain.URL+"/.well-known/oauth-protected-resource", ""); status != 404 {
		t.Errorf("no provider: HTTP %d, want 404", status)
	}
}

// Security is per operation, always said: hello is the one that is open, everything else names its scope.
func TestEveryOperationDeclaresItsSecurity(t *testing.T) {
	for _, op := range humaworkers.New(config(), Routes(Env{})).Operations() {
		switch {
		case op.Security == nil:
			t.Errorf("%s does not declare its Security", op.OperationID)
		case len(op.Security) == 0 && op.OperationID != "hello":
			t.Errorf("%s is open to anyone", op.OperationID)
		}
	}
}

// fern/generators.yml restates the security schemes for Fern: the same headers and variables.
func TestFernAuthMatchesTheContract(t *testing.T) {
	yml, err := os.ReadFile("../fern/generators.yml")
	if err != nil {
		t.Fatal(err)
	}
	for name, scheme := range securitySchemes() {
		want := []string{"    " + name + ":\n"}
		if scheme.Type == "apiKey" {
			fern := scheme.Extensions["x-fern-header"].(map[string]any)
			want = append(want, "header: "+scheme.Name+"\n", "name: "+fern["name"].(string)+"\n", "env: "+fern["env"].(string)+"\n")
		}
		for _, w := range want {
			if !strings.Contains(string(yml), w) {
				t.Errorf("fern/generators.yml: the scheme %s has no %q", name, strings.TrimSpace(w))
			}
		}
	}
	if !strings.Contains(string(yml), "endpoint-security: {}") {
		t.Error("fern/generators.yml: auth is not endpoint-security")
	}
}
