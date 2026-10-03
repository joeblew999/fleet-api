package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testRead  = "test-read-token"
	testWrite = "test-write-token"
	exampleID = "3f9a1c0b7d2e4a65"
)

// A real HTTP server over the in-memory store: the handlers as `go run .` serves them.
func server(t *testing.T) (*httptest.Server, *MemStore) {
	t.Helper()
	memory := &MemStore{}
	settings := map[string]string{"APP_NAME": "test", ReadToken: testRead, WriteToken: testWrite}
	env := Env{
		Var:   func(name string) string { return settings[name] },
		Store: func() (Store, error) { return memory, nil },
	}
	srv := httptest.NewServer(Handler(env))
	t.Cleanup(srv.Close)
	return srv, memory
}

// do sends a request; headers are name, value pairs. "token" as a name is shorthand for
// Authorization: Bearer.
func do(t *testing.T, method, url, body string, headers ...string) (int, string, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		if headers[i] == "token" {
			req.Header.Set("Authorization", "Bearer "+headers[i+1])
		} else {
			req.Header.Set(headers[i], headers[i+1])
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header
}

// edit returns the example report with changes made to it.
func edit(t *testing.T, change func(m map[string]any)) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(ExampleReport, &m); err != nil {
		t.Fatal(err)
	}
	if change != nil {
		change(m)
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func section(m map[string]any, name string) map[string]any { return m[name].(map[string]any) }

func post(t *testing.T, base, body string) (int, string) {
	t.Helper()
	status, answer, _ := do(t, "POST", base+"/api/devices/"+exampleID+"/reports", body, "token", testWrite)
	return status, answer
}

func TestHelloNeedsNoToken(t *testing.T) {
	srv, _ := server(t)
	status, body, _ := do(t, "GET", srv.URL+"/api/hello", "")
	if status != 200 || strings.TrimSpace(body) != `{"message":"Hello from test"}` {
		t.Fatalf("HTTP %d %s", status, body)
	}
}

func TestTheExampleReportIsValid(t *testing.T) {
	var r DeviceReport
	if err := json.Unmarshal(ExampleReport, &r); err != nil {
		t.Fatal(err)
	}
	if problems := r.Validate(); len(problems) > 0 {
		t.Fatal(problems)
	}
}

func TestAReportIsStoredAsPostedAndReadBack(t *testing.T) {
	srv, _ := server(t)
	// A newer tool's field, which the Go type does not have.
	body := edit(t, func(m map[string]any) { m["thermal"] = map[string]any{"status": "ok", "pressure": "nominal"} })
	status, answer := post(t, srv.URL, body)
	if status != 201 || !strings.Contains(answer, `"duplicate":false`) || !strings.Contains(answer, `"conditions":[]`) {
		t.Fatalf("post: %d %s", status, answer)
	}
	if status, answer = post(t, srv.URL, body); status != 201 || !strings.Contains(answer, `"duplicate":true`) {
		t.Fatalf("second post: %d %s", status, answer)
	}
	status, answer, _ = do(t, "GET", srv.URL+"/api/devices/"+exampleID, "", "token", testRead)
	if status != 200 || !strings.Contains(answer, `"pressure":"nominal"`) || !strings.Contains(answer, `"due":`) || !strings.Contains(answer, `"caller":"lead-agent"`) || !strings.Contains(answer, `"session_running":true`) {
		t.Fatalf("get: %d %s", status, answer)
	}
	status, answer, _ = do(t, "GET", srv.URL+"/api/devices", "", "token", testRead)
	if status != 200 || !strings.Contains(answer, `"id":"`+exampleID+`"`) || !strings.Contains(answer, `"now":`) {
		t.Fatalf("list: %d %s", status, answer)
	}
	if status, answer, _ = do(t, "GET", srv.URL+"/api/devices/0000000000000000", "", "token", testRead); status != 404 {
		t.Fatalf("unknown device: %d %s", status, answer)
	}
	if status, answer, _ = do(t, "GET", srv.URL+"/api/devices/0000000000000000/reports", "", "token", testRead); status != 404 {
		t.Fatalf("unknown device's history: %d %s", status, answer)
	}
}

func TestHistoryIsNewestFirstAndTheDeviceFollowsTheNewest(t *testing.T) {
	srv, _ := server(t)
	for _, ts := range []float64{1000, 3000, 2000} { // 2000 arrives last, as a resent report would
		if status, answer := post(t, srv.URL, edit(t, func(m map[string]any) { m["ts"] = ts })); status != 201 {
			t.Fatalf("post %v: %d %s", ts, status, answer)
		}
	}
	var history DeviceHistory
	_, answer, _ := do(t, "GET", srv.URL+"/api/devices/"+exampleID+"/reports?limit=2", "", "token", testRead)
	if err := json.Unmarshal([]byte(answer), &history); err != nil || len(history.Reports) != 2 {
		t.Fatalf("history: %v %s", err, answer)
	}
	var first, second DeviceReport
	json.Unmarshal(history.Reports[0].Report, &first)
	json.Unmarshal(history.Reports[1].Report, &second)
	if first.TS != 3000 || second.TS != 2000 {
		t.Fatalf("history order: %d, %d", first.TS, second.TS)
	}
	_, answer, _ = do(t, "GET", srv.URL+"/api/devices/"+exampleID, "", "token", testRead)
	if !strings.Contains(answer, `"ts":3000`) {
		t.Fatalf("the device's report is not the newest: %s", answer)
	}
}

func TestTokens(t *testing.T) {
	srv, _ := server(t)
	body := edit(t, nil)
	for _, c := range []struct {
		name, method, path, body, token string
		status                          int
	}{
		{"post with no token", "POST", "/api/devices/" + exampleID + "/reports", body, "", 401},
		{"post with the read token", "POST", "/api/devices/" + exampleID + "/reports", body, testRead, 403},
		{"post with a wrong token", "POST", "/api/devices/" + exampleID + "/reports", body, "nope", 401},
		{"post with the write token", "POST", "/api/devices/" + exampleID + "/reports", body, testWrite, 201},
		{"list with no token", "GET", "/api/devices", "", "", 401},
		{"list with the read token", "GET", "/api/devices", "", testRead, 200},
		{"list with the write token", "GET", "/api/devices", "", testWrite, 200},
		{"history with no token", "GET", "/api/devices/" + exampleID + "/reports", "", "", 401},
		{"one device with a wrong token", "GET", "/api/devices/" + exampleID, "", "nope", 401},
		{"delete with no token", "DELETE", "/api/devices/" + exampleID, "", "", 401},
		{"delete with the read token", "DELETE", "/api/devices/" + exampleID, "", testRead, 403},
		{"delete with the write token", "DELETE", "/api/devices/" + exampleID, "", testWrite, 200},
	} {
		headers := []string{}
		if c.token != "" {
			headers = []string{"token", c.token}
		}
		status, answer, header := do(t, c.method, srv.URL+c.path, c.body, headers...)
		if status != c.status || (status == 401 && !strings.HasPrefix(header.Get("WWW-Authenticate"), "Bearer resource_metadata=")) ||
			(status == 403 && !strings.Contains(header.Get("WWW-Authenticate"), `error="insufficient_scope"`)) {
			t.Errorf("%s: HTTP %d %s, want %d", c.name, status, answer, c.status)
		}
	}
}

func TestUnsetTokensMatchNothing(t *testing.T) {
	env := Env{Var: func(string) string { return "" }, Store: func() (Store, error) { return &MemStore{}, nil }}
	srv := httptest.NewServer(Handler(env))
	defer srv.Close()
	for _, token := range []string{"", " "} {
		if status, _, _ := do(t, "GET", srv.URL+"/api/devices", "", "Authorization", "Bearer "+token); status != 401 {
			t.Errorf("token %q with no secrets set: HTTP %d, want 401", token, status)
		}
	}
}

func TestBadReportsAre422WithTheLocation(t *testing.T) {
	srv, _ := server(t)
	for name, c := range map[string]struct {
		change   func(m map[string]any)
		location string
	}{
		"percent 101 (a tag)":               {func(m map[string]any) { section(m, "battery")["percent"] = 101 }, "body.battery.percent"},
		"reason not in the enum (a tag)":    {func(m map[string]any) { m["reason"] = "whenever" }, "body.reason"},
		"upper-case id (a tag)":             {func(m map[string]any) { m["id"] = "3F9A1C0B7D2E4A65" }, "body.id"},
		"no host (a tag)":                   {func(m map[string]any) { delete(m, "host") }, "body"},
		"ok with no percent (Validate)":     {func(m map[string]any) { delete(section(m, "battery"), "percent") }, "body.battery.percent"},
		"unknown with no reason (Validate)": {func(m map[string]any) { m["lid"] = map[string]any{"status": "unknown"} }, "body.lid.why"},
		"schema 2 (Validate)":               {func(m map[string]any) { m["schema"] = 2 }, "body.schema"},
		"interval promising nothing":        {func(m map[string]any) { m["next_s"] = 0 }, "body.next_s"},
		"another device's id (Resolve)":     {func(m map[string]any) { m["id"] = "00aa11bb22cc33dd" }, "body.id"},
		"a home directory in rig.work_dir":  {func(m map[string]any) { section(m, "rig")["work_dir"] = "/Users/someone/claude-work" }, "body.rig.work_dir"},
		"rig ok without logged_in":          {func(m map[string]any) { delete(section(m, "rig"), "logged_in") }, "body.rig.logged_in"},
		"rig unknown with values":           {func(m map[string]any) { m["rig"] = map[string]any{"status": "unknown", "why": "x", "logged_in": true} }, "body.rig.logged_in"},
		"rig status not in the enum":        {func(m map[string]any) { section(m, "rig")["status"] = "fine" }, "body.rig.status"},
		"login ok without logged_in":        {func(m map[string]any) { delete(login(m), "logged_in") }, "body.rig.login.logged_in"},
		"login unknown with no reason":      {func(m map[string]any) { section(m, "rig")["login"] = map[string]any{"status": "unknown"} }, "body.rig.login.why"},
		"login names an account":            {func(m map[string]any) { login(m)["auth_method"] = "someone@example.com" }, "body.rig.login.auth_method"},
		"login expiry known and not known":  {func(m map[string]any) { login(m)["refresh_expires_why"] = "x" }, "body.rig.login.refresh_expires_why"},
		"login though rig is unknown": {func(m map[string]any) {
			m["rig"] = map[string]any{"status": "unknown", "why": "x", "login": map[string]any{"status": "unknown", "why": "y"}}
		}, "body.rig.login"},
		"claims ok without slots":             {func(m map[string]any) { delete(section(m, "claims"), "slots") }, "body.claims.slots"},
		"a caller with an @":                  {func(m map[string]any) { claim(m)["caller"] = "someone@studio-1" }, "body.claims.held[0].caller"},
		"a claim that lapses before it began": {func(m map[string]any) { claim(m)["until"] = 1 }, "body.claims.held[0].until"},
		"a claim id with a space (a tag)":     {func(m map[string]any) { claim(m)["id"] = "a b" }, "body.claims.held[0].id"},
		"more claims than slots":              {func(m map[string]any) { section(m, "claims")["slots"] = 1; held(m, 2) }, "body.claims.held"},
		"vms ok without manager_running":      {func(m map[string]any) { delete(section(m, "vms"), "manager_running") }, "body.vms.manager_running"},
		"vms unknown with a list": {func(m map[string]any) {
			section(m, "vms")["status"] = "unknown"
			section(m, "vms")["why"] = "x"
			delete(section(m, "vms"), "manager_running")
			delete(section(m, "vms"), "manager")
		}, "body.vms.list"},
		"a VM owner with an @":              {func(m map[string]any) { vm(m)["owner"] = "someone@studio-1:repo" }, "body.vms.list[0].owner"},
		"a VM named twice":                  {func(m map[string]any) { vm(m)["name"] = "IRGO-GOLDEN" }, "body.vms.list[1].name"},
		"a VM's os not in the enum (a tag)": {func(m map[string]any) { vm(m)["os"] = "plan9" }, "body.vms.list[0].os"},
	} {
		status, answer := post(t, srv.URL, edit(t, c.change))
		if status != 422 || !strings.Contains(answer, `"location":"`+c.location+`"`) {
			t.Errorf("%s: %d %s", name, status, answer)
		}
	}
	if status, answer := post(t, srv.URL, edit(t, func(m map[string]any) { m["pad"] = strings.Repeat("x", DeviceMaxBody) })); status != 413 {
		t.Errorf("over 16 KiB: %d %s", status, answer)
	}
}

func TestRigClaimsAndVMsAreOptional(t *testing.T) {
	srv, _ := server(t)
	body := edit(t, func(m map[string]any) { delete(m, "rig"); delete(m, "claims"); delete(m, "vms") })
	if status, answer := post(t, srv.URL, body); status != 201 {
		t.Fatalf("without rig, claims and vms: %d %s", status, answer)
	}
	body = edit(t, func(m map[string]any) {
		m["ts"] = 1790842406356
		m["rig"] = map[string]any{"status": "none"}
		m["claims"] = map[string]any{"status": "unknown", "why": "the claims folder is not readable"}
		m["vms"] = map[string]any{"status": "none"}
	})
	if status, answer := post(t, srv.URL, body); status != 201 {
		t.Fatalf("rig none, claims unknown, vms none: %d %s", status, answer)
	}
	// The manager closed: the VMs the tool knows of, every one stopped.
	body = edit(t, func(m map[string]any) {
		m["ts"] = 1790842406357
		section(m, "vms")["manager_running"] = false
	})
	if status, answer := post(t, srv.URL, body); status != 201 {
		t.Fatalf("vms with the manager closed: %d %s", status, answer)
	}
}

func login(m map[string]any) map[string]any { return section(section(m, "rig"), "login") }

func vm(m map[string]any) map[string]any {
	return section(m, "vms")["list"].([]any)[0].(map[string]any)
}

func claim(m map[string]any) map[string]any {
	return section(m, "claims")["held"].([]any)[0].(map[string]any)
}

func held(m map[string]any, n int) {
	var list []any
	for i := range n {
		list = append(list, map[string]any{"id": "c" + itoa(i), "caller": "agent", "job": "work", "since": 1})
	}
	section(m, "claims")["held"] = list
}

func TestAMachineCanBeForgotten(t *testing.T) {
	srv, _ := server(t)
	other := strings.ReplaceAll(edit(t, nil), exampleID, "00aa11bb22cc33dd")
	for _, body := range []string{edit(t, nil), edit(t, func(m map[string]any) { m["ts"] = 1 })} {
		if status, answer := post(t, srv.URL, body); status != 201 {
			t.Fatalf("post: %d %s", status, answer)
		}
	}
	if status, answer, _ := do(t, "POST", srv.URL+"/api/devices/00aa11bb22cc33dd/reports", other, "token", testWrite); status != 201 {
		t.Fatalf("post the other: %d %s", status, answer)
	}
	status, answer, _ := do(t, "DELETE", srv.URL+"/api/devices/"+exampleID, "", "token", testWrite)
	if status != 200 || strings.TrimSpace(answer) != `{"id":"`+exampleID+`","reports":2}` {
		t.Fatalf("delete: %d %s", status, answer)
	}
	if status, answer, _ = do(t, "GET", srv.URL+"/api/devices/"+exampleID, "", "token", testRead); status != 404 {
		t.Fatalf("get after delete: %d %s", status, answer)
	}
	if status, answer, _ = do(t, "GET", srv.URL+"/api/devices/"+exampleID+"/reports", "", "token", testRead); status != 404 {
		t.Fatalf("history after delete: %d %s", status, answer)
	}
	if _, answer, _ = do(t, "GET", srv.URL+"/api/devices", "", "token", testRead); strings.Contains(answer, exampleID) || !strings.Contains(answer, "00aa11bb22cc33dd") {
		t.Fatalf("list after delete: %s", answer)
	}
	if status, answer, _ = do(t, "DELETE", srv.URL+"/api/devices/"+exampleID, "", "token", testWrite); status != 404 {
		t.Fatalf("delete again: %d %s", status, answer)
	}
	// A machine that reports again comes back.
	if status, answer := post(t, srv.URL, edit(t, nil)); status != 201 || !strings.Contains(answer, `"duplicate":false`) {
		t.Fatalf("post after delete: %d %s", status, answer)
	}
}

func TestConditions(t *testing.T) {
	srv, _ := server(t)
	ts := 0
	for name, c := range map[string]struct {
		change func(m map[string]any)
		code   string
	}{
		"lid does nothing, no keeper": {func(m map[string]any) { section(m, "sleep")["lid_action"] = "nothing" }, "unminded"},
		"on battery under 20": {func(m map[string]any) {
			section(m, "power")["source"] = "battery"
			section(m, "battery")["percent"], section(m, "battery")["state"] = 12, "discharging"
		}, "battery-low"},
		"closed on battery": {func(m map[string]any) {
			section(m, "power")["source"] = "battery"
			section(m, "battery")["state"] = "discharging"
			section(m, "lid")["closed"] = true
		}, "closed-on-battery"},
		"stopped":           {func(m map[string]any) { m["reason"], m["next_s"] = "stop", 0 }, "stopped"},
		"Claude logged out": {func(m map[string]any) { section(m, "rig")["logged_in"] = false }, "rig-unready"},
	} {
		status, answer := post(t, srv.URL, edit(t, func(m map[string]any) { ts++; m["ts"] = ts; c.change(m) }))
		if status != 201 || !strings.Contains(answer, `"code":"`+c.code+`"`) {
			t.Errorf("%s: %d %s", name, status, answer)
		}
	}
}

func TestQuietIsJudgedByWhenTheWorkerReceivedIt(t *testing.T) {
	var r DeviceReport
	json.Unmarshal(ExampleReport, &r)
	if got := conditions(r, 0, 3*r.NextS*1000); len(got) != 0 {
		t.Fatalf("on time: %v", got)
	}
	got := conditions(r, 0, 3*r.NextS*1000+1)
	if len(got) != 1 || got[0].Code != DeviceCondQuiet || got[0].Since != 3*r.NextS*1000 {
		t.Fatalf("late: %v", got)
	}
}

func TestUnknownPathAndMethod(t *testing.T) {
	srv, _ := server(t)
	if status, _, _ := do(t, "GET", srv.URL+"/api/nope", ""); status != 404 {
		t.Errorf("unknown path: HTTP %d, want 404", status)
	}
	status, _, header := do(t, "DELETE", srv.URL+"/api/devices", "")
	if status != 405 || header.Get("Allow") != "GET" {
		t.Errorf("DELETE /api/devices: HTTP %d Allow %q, want 405 GET", status, header.Get("Allow"))
	}
}

func TestHomeDirectories(t *testing.T) {
	for path, home := range map[string]bool{
		"~/claude-work": false, "/System/Volumes/Data": false, "/": false, "C:\\": false, "/Users/Shared/work": false,
		"/Users/someone": true, "/home/someone/work": true, "C:\\Users\\someone\\work": true,
	} {
		if homeDir(path) != home {
			t.Errorf("homeDir(%q) = %v", path, !home)
		}
	}
}

// ---- the spec ----

func TestTheWorkerServesTheSpecWithItsOriginAsServer(t *testing.T) {
	srv, _ := server(t)
	_, openapi, _ := do(t, "GET", srv.URL+"/api/openapi.json", "")
	if !strings.Contains(openapi, `"servers":[{"url":"`+srv.URL+`"}]`) {
		t.Errorf("openapi servers: %s", openapi)
	}
	for _, want := range []string{`"name":"CF-Access-Client-Id"`, `"name":"CF-Access-Client-Secret"`, `"openIdConnectUrl":"/.well-known/openid-configuration"`, `"scheme":"bearer"`,
		`"security":[{"accessClientId":["devices:write"],"accessClientSecret":[]},{"oidc":["devices:write"]},{"bearer":["devices:write"]}]`} {
		if !strings.Contains(openapi, want) {
			t.Errorf("the spec has no %s", want)
		}
	}
}
