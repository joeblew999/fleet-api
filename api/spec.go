package api

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/asyncapi"
	"github.com/joeblew999/charter/go/humaworkers"
)

func init() {
	// A list is [] when empty, never null: generated SDKs then type it as a plain array.
	huma.DefaultArrayNullable = false
}

func config() huma.Config {
	config := humaworkers.Config(Title, Version)
	config.Info.Description = Description
	// Who may call what (auth.go): the schemes here; each operation names the scope it needs on
	// them (TestEveryOperationDeclaresItsSecurity). No API-wide default: Fern's importer drops an
	// operation's security when it equals the default, and its SDKs then send no credentials there.
	config.Components.SecuritySchemes = securitySchemes()
	return config
}

// OpenAPI is the OpenAPI spec of the contract, with server as its server: what cmd/spec writes
// for Fern and what the Worker serves at /api/openapi.json.
func OpenAPI(server string) ([]byte, error) {
	config := config()
	config.Servers = []*huma.Server{{URL: server}}
	return json.Marshal(humaworkers.New(config, Routes(Env{})).OpenAPI())
}

// AsyncAPI is the AsyncAPI spec of the contract's WebSocket channels (/api/asyncapi.json).
func AsyncAPI(server string) ([]byte, error) {
	api := humaworkers.New(config(), Routes(Env{}))
	doc, err := asyncapi.Generate(api.Operations(), api.OpenAPI().Components.Schemas, asyncapi.Info{Title: LiveTitle, Version: Version}, server)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}
