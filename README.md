# fleet-api

One place that knows every machine in the fleet, what it is, whether it is healthy, and who is using it — readable from anywhere, written by the machines themselves.

Machines post a report about themselves; anyone with the read token reads the fleet. A Go API on
Cloudflare Workers (`https://fleet-api.gedw99.workers.dev`), made with `charter new` from
[charter](https://github.com/joeblew999/charter), with a Go SDK in `sdk/go/`.

```sh
mise install && mise run setup     # tools, then npm packages
mise run check                     # every local check
mise run run                       # natively: http://localhost:5174/api/hello
mise run dev                       # under workerd (first time: mise run migrate:local)
mise run deploy                    # to Cloudflare, then: mise run live-test
mise tasks                         # everything else: every task is one line
```

The contract is `api/contract.go` and `api/device.go`. After changing it: `mise run spec`. Docs are in [docs/](docs/README.md).
