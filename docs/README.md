---
title: Start here
nav_order: 1
permalink: /
---

# fleet-api

Everything written about this project lives in this folder. `AGENTS.md` only points here.

| Page | What it covers |
|---|---|
| This page | What is what |
| [rules.md](rules.md) | The working rules |
| [writing.md](writing.md) | The rules a page in `docs/` is held to |

## What is what

You write the contract in Go; everything else is generated from it.

| | |
|---|---|
| **The contract (the source; you edit this)** | `api/contract.go` (Huma: Go structs and their tags) |
| The server | `api/handlers.go` |
| The Worker's entry | `worker.mjs`. The rest of the JavaScript is the Go library's: the build writes it into `build/` |
| Write the specs | `mise run spec` |
| **The specs (generated; never edit)** | `fern/openapi.json`, `fern/asyncapi.json` |
| Fern's settings | `fern/generators.yml` |
| Generate an SDK | `mise run sdk:gen <group>` (go, typescript, typescript-dist, cli) into `sdk/out/` |
| The D1 schema | `migrations/` |
| The tests a deploy must pass | `test/` (`mise run live-test`, `mise run soak`) |
| MCP | `/api/mcp`: every one-shot operation of the contract is a tool |
| The tasks | `mise.toml`: each is one line, and what needs more is a command of the charter tool it pins |

The project starts as the notes example. Which files hold it, and what to do with each when you put your
own API in: [Replace the example with your API](https://joeblew999.github.io/charter/guides/replace-the-example.html).

How it works, what Huma needs on workers-go, the real-time design and the measured costs are documented
once, in [charter's docs](https://joeblew999.github.io/charter/): the Go library this project requires
(`github.com/joeblew999/charter/go`: `humaworkers`, `asyncapi`, `follow`, `hub`, `d1`, `humamcp`, `transport`, `specfile`, and the Worker
glue the build writes into `build/`) comes from there.
