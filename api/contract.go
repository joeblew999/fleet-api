// Package api is the fleet's device API, contract first, in Go. Every route is a Huma operation whose
// input and output are Go structs; the struct tags are the schema (device.go holds the report). From
// this one definition come the handlers' validation, the OpenAPI spec (spec.go), and from it Fern's
// SDKs and CLI. OperationID and Tags name the SDK methods, and Extensions carry Fern's x-fern-*.
// The `example` tags, and the example report, are what the SDKs' READMEs and references show
// (TestExamplesAreThereAndValid).
package api

import (
	_ "embed"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/humamcp"
	"github.com/joeblew999/charter/go/humaworkers"
)

// What the specs say about the API as a whole.
const (
	Title       = "fleet-api"
	Version     = "1.0.0"
	Description = "One place that knows every machine in the fleet, what it is, whether it is healthy, and who is using it: readable from anywhere, written by the machines themselves."
	LiveTitle   = "fleet-api live"
)

// ExampleReport is a valid report: the example the specs, and so the SDKs' docs, show.
//
//go:embed example_report.json
var ExampleReport []byte

type HelloOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello from fleet-api"`
	}
}

type DevicePostInput struct {
	ID      string `path:"id" pattern:"^[0-9a-f]{16}$" example:"3f9a1c0b7d2e4a65" doc:"The machine id; must be the report's id"`
	Body    DeviceReport
	RawBody []byte // the report as posted: what is stored, so a newer tool's fields survive
}

type DevicePostOutput struct{ Body DevicePosted }

type DeviceGetInput struct {
	ID string `path:"id" pattern:"^[0-9a-f]{16}$" example:"3f9a1c0b7d2e4a65" doc:"The machine id: 16 lower-case hex digits"`
}

type DeviceOutput struct{ Body DeviceView }

type DevicesOutput struct{ Body DeviceList }

type DeviceReportsInput struct {
	ID string `path:"id" pattern:"^[0-9a-f]{16}$" example:"3f9a1c0b7d2e4a65" doc:"The machine id: 16 lower-case hex digits"`
	// The example is small: under TinyGo (32-bit int) Huma panics on an int tag over 2^31 (a time in ms).
	Since int64 `query:"since" minimum:"0" example:"0" doc:"Only reports received at or after this, Unix milliseconds; 0: all that are kept"`
	Limit int32 `query:"limit" minimum:"1" maximum:"500" default:"50" example:"50" doc:"At most this many, newest first"`
}

type DeviceReportsOutput struct{ Body DeviceHistory }

type DeviceDeleteInput struct {
	ID string `path:"id" pattern:"^[0-9a-f]{16}$" example:"3f9a1c0b7d2e4a65" doc:"The machine id: 16 lower-case hex digits"`
}

type DeviceDeleteOutput struct{ Body DeviceDeleted }

// sdk is Fern's names for the SDK method: client.<group>.<method>() and `cli <group> <method>`.
func sdk(group, method string, extra map[string]any) map[string]any {
	extensions := map[string]any{"x-fern-sdk-group-name": group, "x-fern-sdk-method-name": method}
	for key, value := range extra {
		extensions[key] = value
	}
	return extensions
}

// Routes is the contract with its implementation on env.
func Routes(env Env) []humaworkers.Route {
	return []humaworkers.Route{
		{Method: http.MethodGet, Path: "/api/hello", OperationID: "hello", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "hello", Method: http.MethodGet, Path: "/api/hello",
				Summary: "Say hello: the one operation that needs no credentials", Tags: []string{"meta"},
				Security:   []map[string][]string{},
				Extensions: sdk("meta", "hello", nil),
			}, env.hello)
		}},
		{Method: http.MethodPost, Path: "/api/devices/{id}/reports", OperationID: "postDeviceReport", Register: func(api huma.API) {
			// Not an MCP tool: the machines write, each for itself; agents read.
			huma.Register(api, humamcp.Expose(huma.Operation{
				OperationID: "postDeviceReport", Method: http.MethodPost, Path: "/api/devices/{id}/reports",
				Summary:     "Post a machine's report (devices:write: a machine's service token, for its own id)",
				Description: "Stored as posted. The same id and ts again is a duplicate and changes nothing, so a machine can resend what it could not deliver. A machine's service token posts only for the device it was enrolled for: another id is 403.",
				Tags:        []string{"devices"}, DefaultStatus: http.StatusCreated,
				MaxBodyBytes: DeviceMaxBody,
				Security:     requires(ScopeWrite),
				Errors:       []int{http.StatusForbidden, http.StatusRequestEntityTooLarge},
				Extensions:   sdk("devices", "report", nil),
			}, false), env.devicePost)
			if schema := api.OpenAPI().Components.Schemas.Map()["DeviceReport"]; schema != nil && len(schema.Examples) == 0 {
				var example any
				if json.Unmarshal(ExampleReport, &example) == nil {
					schema.Examples = []any{example}
				}
			}
		}},
		{Method: http.MethodGet, Path: "/api/devices", OperationID: "listDevices", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "listDevices", Method: http.MethodGet, Path: "/api/devices",
				Summary: "Every machine as last heard from, with its conditions", Tags: []string{"devices"},
				Security: requires(ScopeRead), Errors: []int{http.StatusForbidden},
				Extensions: sdk("devices", "list", nil),
			}, env.deviceList)
		}},
		{Method: http.MethodGet, Path: "/api/devices/{id}", OperationID: "getDevice", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "getDevice", Method: http.MethodGet, Path: "/api/devices/{id}",
				Summary: "One machine as last heard from, with its conditions", Tags: []string{"devices"},
				Security: requires(ScopeRead), Errors: []int{http.StatusForbidden, http.StatusNotFound},
				Extensions: sdk("devices", "get", nil),
			}, env.deviceGet)
		}},
		{Method: http.MethodGet, Path: "/api/devices/{id}/reports", OperationID: "listDeviceReports", Register: func(api huma.API) {
			huma.Register(api, huma.Operation{
				OperationID: "listDeviceReports", Method: http.MethodGet, Path: "/api/devices/{id}/reports",
				Summary: "A machine's reports of the last 7 days, newest first", Tags: []string{"devices"},
				Security: requires(ScopeRead), Errors: []int{http.StatusForbidden, http.StatusNotFound},
				Extensions: sdk("devices", "history", nil),
			}, env.deviceReports)
		}},
		{Method: http.MethodDelete, Path: "/api/devices/{id}", OperationID: "deleteDevice", Register: func(api huma.API) {
			// Not an MCP tool: it changes data; a person does it.
			huma.Register(api, humamcp.Expose(huma.Operation{
				OperationID: "deleteDevice", Method: http.MethodDelete, Path: "/api/devices/{id}",
				Summary:     "Forget a machine and its reports (devices:forget: a person logged in through Access)",
				Description: "For a machine that is gone, or an id no longer used. A machine that reports again comes back.",
				Tags:        []string{"devices"},
				Security:    requires(ScopeForget),
				Errors:      []int{http.StatusForbidden, http.StatusNotFound},
				Extensions:  sdk("devices", "delete", nil),
			}, false), env.deviceDelete)
		}},
	}
}
