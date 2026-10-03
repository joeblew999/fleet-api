package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/joeblew999/charter/go/humaworkers"
)

// mcp POSTs one JSON-RPC request to /api/mcp, with the read token, and returns the HTTP status and
// the decoded answer.
func mcp(t *testing.T, base, method, params string) (int, map[string]any) {
	t.Helper()
	return mcpAs(t, base, testRead, method, params)
}

func mcpAs(t *testing.T, base, token, method, params string) (int, map[string]any) {
	t.Helper()
	status, body, _ := do(t, "POST", base+"/api/mcp", `{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":`+params+`}`, "token", token)
	var answer map[string]any
	if err := json.Unmarshal([]byte(body), &answer); err != nil {
		t.Fatalf("%s: HTTP %d %s", method, status, body)
	}
	return status, answer
}

// toolCall is a tool call's result: its text, structuredContent and isError.
func toolCall(t *testing.T, base, token, name, arguments string) (string, any, bool) {
	t.Helper()
	status, answer := mcpAs(t, base, token, "tools/call", `{"name":"`+name+`","arguments":`+arguments+`}`)
	result, _ := answer["result"].(map[string]any)
	if status != 200 || result == nil {
		t.Fatalf("tools/call %s: HTTP %d %v", name, status, answer)
	}
	text := ""
	for _, block := range result["content"].([]any) {
		text += block.(map[string]any)["text"].(string)
	}
	return text, result["structuredContent"], result["isError"] == true
}

// The tools are the operations that read, and hello; posting a report and forgetting a machine are
// not: they change data, with the write token.
func TestMCPToolsAreTheReadOperations(t *testing.T) {
	srv, _ := server(t)
	status, answer := mcp(t, srv.URL, "tools/list", `{}`)
	if status != 200 {
		t.Fatalf("HTTP %d %v", status, answer)
	}
	result, _ := answer["result"].(map[string]any)
	listed, _ := result["tools"].([]any)
	var names []string
	byName := map[string]map[string]any{}
	for _, tool := range listed {
		name, _ := tool.(map[string]any)["name"].(string)
		names = append(names, name)
		byName[name] = tool.(map[string]any)
	}
	if strings.Join(names, ",") != "hello,listDevices,getDevice,listDeviceReports" {
		t.Fatalf("tools: %v", names)
	}
	input, _ := json.Marshal(byName["listDeviceReports"]["inputSchema"])
	if !strings.Contains(string(input), `"limit":{`) || !strings.Contains(string(input), `"since":{`) || !strings.Contains(string(input), `"id":{`) {
		t.Errorf("listDeviceReports input: %s", input)
	}
}

func TestMCPCallsRunTheSameOperationsAsREST(t *testing.T) {
	srv, _ := server(t)
	status, answer := mcp(t, srv.URL, "initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}`)
	want := map[string]any{
		"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}},
		"serverInfo": map[string]any{"name": Title, "version": Version}, "instructions": Description,
	}
	if status != 200 || !reflect.DeepEqual(answer["result"], want) {
		t.Fatalf("initialize: HTTP %d %v, want %v", status, answer["result"], want)
	}
	if text, _, isError := toolCall(t, srv.URL, "", "hello", `{}`); text != `{"message":"Hello from test"}` || isError {
		t.Fatalf("hello: %s", text)
	}
	if status, answer := post(t, srv.URL, edit(t, nil)); status != 201 {
		t.Fatalf("post: %d %s", status, answer)
	}
	text, structured, isError := toolCall(t, srv.URL, testRead, "getDevice", `{"id":"`+exampleID+`"}`)
	if isError || structured.(map[string]any)["report"].(map[string]any)["id"] != exampleID {
		t.Fatalf("getDevice: %s", text)
	}
	text, _, isError = toolCall(t, srv.URL, testRead, "listDevices", `{}`)
	// The same answer as the REST route, but for the Worker's clock.
	_, rest, _ := do(t, "GET", srv.URL+"/api/devices", "", "token", testRead)
	if isError || !strings.Contains(text, `"id":"`+exampleID+`"`) || len(text) != len(strings.TrimSpace(rest)) {
		t.Fatalf("listDevices over MCP: %s\nover REST: %s", text, rest)
	}
}

func TestMCPErrors(t *testing.T) {
	srv, _ := server(t)
	// The token is the caller's: a tool call without one is refused as the REST route refuses it.
	if text, _, isError := toolCall(t, srv.URL, "", "listDevices", `{}`); !isError || !strings.Contains(text, `"status":401`) {
		t.Errorf("no token: %s", text)
	}
	// What the contract refuses is a tool error carrying Huma's problem, with the location.
	for name, test := range map[string]struct{ tool, arguments, location string }{
		"a path parameter that is not an id": {"getDevice", `{"id":"x"}`, `"location":"path.id"`},
		"a query parameter out of range":     {"listDeviceReports", `{"id":"` + exampleID + `","limit":1000}`, `"location":"query.limit"`},
	} {
		text, structured, isError := toolCall(t, srv.URL, testRead, test.tool, test.arguments)
		if !isError || !strings.Contains(text, test.location) || !strings.Contains(text, `"status":422`) || structured != nil {
			t.Errorf("%s: isError %v, %s; want an error with %s", name, isError, text, test.location)
		}
	}
	code := func(answer map[string]any) any {
		failure, _ := answer["error"].(map[string]any)
		return failure["code"]
	}
	for name, test := range map[string]struct {
		method, params string
		code           float64
	}{
		"unknown tool":           {"tools/call", `{"name":"noSuchTool","arguments":{}}`, -32602},
		"posting is not a tool":  {"tools/call", `{"name":"postDeviceReport","arguments":{}}`, -32602},
		"deleting is not a tool": {"tools/call", `{"name":"deleteDevice","arguments":{"id":"3f9a1c0b7d2e4a65"}}`, -32602},
		"unknown method":         {"resources/list", `{}`, -32601},
	} {
		if status, answer := mcp(t, srv.URL, test.method, test.params); status != 200 || code(answer) != test.code {
			t.Errorf("%s: HTTP %d %v, want error %v", name, status, answer, test.code)
		}
	}
	if status, _, header := do(t, "GET", srv.URL+"/api/mcp", ""); status != 405 || header.Get("Allow") != "POST" {
		t.Errorf("GET: HTTP %d Allow %q, want 405 POST", status, header.Get("Allow"))
	}
}

// Route.OperationID is how a tool call finds its one operation: it must be the operation's own id.
func TestEveryRouteNamesItsOperation(t *testing.T) {
	for _, route := range Routes(Env{}) {
		ops := humaworkers.New(config(), []humaworkers.Route{route}).Operations()
		if len(ops) != 1 || route.OperationID == "" || ops[0].OperationID != route.OperationID || ops[0].Method != route.Method || ops[0].Path != route.Path {
			t.Errorf("route %s %s (%q) registers %+v", route.Method, route.Path, route.OperationID, ops)
		}
	}
}

// withoutExamples removes every "examples" key from a decoded JSON value, in place.
func withoutExamples(v any) {
	switch v := v.(type) {
	case map[string]any:
		delete(v, "examples")
		for _, child := range v {
			withoutExamples(child)
		}
	case []any:
		for _, child := range v {
			withoutExamples(child)
		}
	}
}
