# fleet-api

A contract-first Go API on Cloudflare Workers: Huma on workers-go, built with TinyGo, with real-time
(SSE + WebSocket) and an MCP endpoint. The OpenAPI and AsyncAPI specs are generated from the Go
contract, and Fern generates SDKs and a CLI from them. Made with `charter new` from
[charter](https://github.com/joeblew999/charter) (v0.9.1), whose [docs](https://joeblew999.github.io/charter/) explain the design.

```sh
mise install && mise run setup     # tools, then npm packages
mise run check                     # every local check
mise run run                       # natively: http://localhost:5174/api/hello
mise run dev                       # under workerd (first time: mise run migrate:local)
mise run deploy                    # to Cloudflare, then: mise run live-test
mise tasks                         # everything else: every task is one line
```

The contract is `api/contract.go`. After changing it: `mise run spec`. Docs are in [docs/](docs/README.md).
