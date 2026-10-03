---
title: Home
nav_order: 1
permalink: /
---

# fleet-api

> One place that knows every machine in the fleet, what it is, whether it is healthy, and who is using it — readable from anywhere, written by the machines themselves.

Each machine posts a report about itself; anyone with the read token reads the fleet, from a phone, a script or an agent. A Go API on Cloudflare Workers, made with [charter](https://github.com/joeblew999/charter). Start with [Getting started](getting-started.md).

## What you get

- **One report per machine** ([The report](guides/report.md)): what it is (host, CPU, memory, disks, power, battery, lid, sleep), the Claude worker on it (`rig`) and who holds it (`claims`). Every section says ok, none or unknown, so a zero is always a measured zero.
- **Five routes** ([Routes](reference/routes.md)): post a report, list the machines, one machine with its conditions, a machine's history of the last 7 days, forget a machine.
- **Two tokens** ([Tokens](guides/tokens.md)): machines write, readers read.
- **A Go SDK** another repo can `go get` ([The Go SDK](guides/go-sdk.md)), a TypeScript SDK that checks a report against the schema before it posts it ([Post a report](guides/report.md#post-one-from-a-machine)), and the read routes as MCP tools at `/api/mcp`.

Deployed at `https://fleet-api.gedw99.workers.dev`.

## What is where

| | |
|---|---|
| **The contract (the source)** | `api/contract.go` (routes), `api/device.go` (the report and its rules) |
| The server | `api/handlers.go` (tokens, conditions), `api/store.go` and `api/store_js.go` (memory, D1) |
| The D1 schema | `migrations/` |
| The tests | `api/*_test.go`; `test/` (run natively, under workerd and against Cloudflare) |
| Fern's settings | `fern/generators.yml` |
| Where the tokens are | `fnox.toml` names them; the values are in the macOS keychain |
| The tasks | `mise.toml`: each is one line |

## What is generated

Never edit these: change the source and run the task.

| Path | Written by |
|---|---|
| `fern/openapi.json`, `fern/asyncapi.json` | `mise run spec`, from the contract |
| `sdk/go/` (committed), `sdk/out/` | `mise run sdk:publish`, `mise run sdk:gen` |
| `build/` | `mise run build` |
| `.github/` | `mise run workflows` |
| `docs/_config.yml`, `docs/writing.md`, `docs/llms.txt`, `docs/_sass/` | `mise run docs:setup` |

## Every page

| Section | Pages |
|---|---|
| Start | [Getting started](getting-started.md) |
| [Guides](guides.md) | [The report](guides/report.md), [Tokens](guides/tokens.md), [The Go SDK](guides/go-sdk.md) |
| [Reference](reference.md) | [Routes](reference/routes.md) |
| [How to help](contributing.md) | [Rules](rules.md), [Benchmarks](benchmarks.md), [Writing docs](writing.md) |

How the project is built (Huma on workers-go, TinyGo, Fern) is documented once, in [charter's docs](https://joeblew999.github.io/charter/).
