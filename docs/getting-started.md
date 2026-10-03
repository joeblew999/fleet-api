---
title: Getting started
nav_order: 2
---

# Getting started: run it, deploy it, report a machine

You need [mise](https://mise.jdx.dev), Go and git; from step 3 Docker, a Cloudflare account and [fnox](https://github.com/jdx/fnox) with the Cloudflare credentials (`CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`).

## 1. Install and check

```sh
git clone https://github.com/joeblew999/fleet-api && cd fleet-api
mise install && mise run setup    # tools, then npm packages
mise run check                    # lint, tests, spec drift, the TinyGo build, the live and MCP tests natively and under workerd
```

The local checks use throwaway tokens (`local-read-token`, `local-write-token`, set in `mise.toml`).

## 2. Run it and post a report

```sh
mise run run      # natively, in memory: http://localhost:5174
curl -X POST localhost:5174/api/devices/3f9a1c0b7d2e4a65/reports \
  -H 'authorization: Bearer local-write-token' -H 'content-type: application/json' \
  --data-binary @api/example_report.json
curl -H 'authorization: Bearer local-read-token' localhost:5174/api/devices
```

What a report holds: [The report](guides/report.md). Every route: [Routes](reference/routes.md).

## 3. Deploy

```sh
mise run build && mise run tokens:put    # once: makes the tokens' Worker secrets (a first deploy needs them)
mise run deploy                          # the Worker, its D1 database, the migrations
mise run live-test                       # always after a deploy: every route, the TypeScript SDK and MCP, on Cloudflare
```

`tokens:put` reads the two tokens from fnox; making them: [Tokens](guides/tokens.md). The live test reports as a test machine, `0000000000000001` (host `live-test`), which stays in the list.

## 4. Read the fleet from anywhere

```sh
fnox exec -- sh -c 'curl -s -H "authorization: Bearer $READ_TOKEN" https://fleet-api.gedw99.workers.dev/api/devices'
```

From Go: [The Go SDK](guides/go-sdk.md). From an agent: the read routes are MCP tools at `/api/mcp`, with the read token as `Authorization: Bearer`.

## 5. On GitHub

```sh
mise run cloudflare:secrets    # once: the deploy workflow's two Cloudflare secrets, from fnox
mise run docs:pages            # once: GitHub Pages for docs/
```
