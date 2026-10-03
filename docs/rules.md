---
title: Rules
nav_order: 2
---

# Rules for working in this project

- **`docs/` is the single source of truth.** Write things down in a page here, following [writing.md](writing.md). `mise run docs:lint` checks what a program can, `mise run docs:review` has Claude check the rest.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything longer belongs in the charter tool the tasks call.
- **The contract is the source.** After changing `api/contract.go`, run `mise run spec`. `mise run check` fails if a committed spec is stale. Never edit `fern/openapi.json` or `fern/asyncapi.json` by hand.
- **Everything that ships to Workers builds with TinyGo** (`mise run build`). `go test` can't see TinyGo's gaps, so the check also runs the Wasm under workerd.
- **Test locally and on Cloudflare.** After a deploy, `mise run live-test` must pass against the deployed Worker: some bugs exist only in production.
- **Exact pins.** Tools in `mise.toml`, Go modules in `go.mod`, npm packages in `package.json`. Lockfiles are committed.
- **Workarounds name their upstream issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code; `mise run upstream:status` lists them.
