//go:build !(js && wasm)

package main

import (
	"os"
	"strings"

	"github.com/joeblew999/fleet-api/api"
)

var memory = &api.MemStore{}

// env is one in-memory store for the process, and the environment's settings: READ_TOKEN,
// WRITE_TOKEN, the issuers it trusts. MACHINES enrols service tokens in the store, as mise run
// machine:enrol does in D1 ("<client id>=<device id>", space-separated): what the local auth test
// posts with.
func env() api.Env {
	for _, pair := range strings.Fields(os.Getenv("MACHINES")) {
		if token, device, ok := strings.Cut(pair, "="); ok {
			memory.Enrol(token, device)
		}
	}
	return api.Env{
		Var: func(name string) string {
			if name == "APP_NAME" && os.Getenv(name) == "" {
				return "fleet-api (go run)"
			}
			return os.Getenv(name)
		},
		Store: func() (api.Store, error) { return memory, nil },
	}
}
