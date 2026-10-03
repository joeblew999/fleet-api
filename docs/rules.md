---
title: Rules
nav_order: 1
parent: How to help
---

# Rules for working in this project

- **`docs/` is the single source of truth.** Write things down in a page here, following [writing.md](writing.md). `mise run docs:lint` checks what a program can, `mise run docs:review` has Claude check the rest.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything longer belongs in the charter tool the tasks call, or a program in `scripts/` or `test/`.
- **The contract is the source.** After changing `api/contract.go` or `api/device.go`, run `mise run spec`, then `mise run sdk:publish`. `mise run check` fails if a committed spec or `sdk/go/` is stale. Never edit `fern/openapi.json`, `fern/asyncapi.json` or `sdk/go/` by hand.
- **Nothing that identifies a person or a network goes into a report:** no serial number, MAC or IP address, or user name; a home directory is `~`. The Worker refuses what it can recognise ([The report](guides/report.md)).
- **Tokens are never printed or committed.** Their values are in the keychain (fnox), the Worker's secrets and each machine's own file ([Access](guides/access.md), [Tokens](guides/tokens.md)).
- **Who may call what is in the contract.** Each operation's `Security` names its scope, and charter's `go/auth` enforces it; the one rule beside it is that a machine posts only for its own device (`MayPostFor`, [Auth and authz](concepts/auth.md)).
- **Everything that ships to Workers builds with TinyGo** (`mise run build`). `go test` can't see TinyGo's gaps, so the check also runs the Wasm under workerd.
- **Test locally and on Cloudflare.** After a deploy, `mise run live-test` must pass against the deployed Worker: some bugs exist only in production.
- **Only verified results go into the docs:** what ran, where, when. Numbers only in [Benchmarks](benchmarks.md).
- **Exact pins.** Tools in `mise.toml`, Go modules in `go.mod`, npm packages in `package.json`. Lockfiles are committed.
- **Workarounds name their upstream issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code; `mise run upstream:status` lists them.
