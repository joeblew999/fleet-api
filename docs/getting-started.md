---
title: Getting started
nav_order: 2
---

# Getting started: run it, deploy it, report a machine

You need [mise](https://mise.jdx.dev), Go and git; from step 3 Docker, a Cloudflare account with Zero Trust (Access, a GitHub login) and [fnox](https://github.com/jdx/fnox) with the Cloudflare credentials ([Access](guides/access.md) lists them).

## 1. Install and check

```sh
git clone https://github.com/joeblew999/fleet-api && cd fleet-api
mise install && mise run setup    # tools, then npm packages
mise run check                    # lint, tests, spec drift, the TinyGo build, the live, MCP and auth tests natively and under workerd
```

The local checks use throwaway tokens (`local-read-token`, `local-write-token`, set in `mise.toml`) and a test issuer they make for Access and OpenID Connect tokens (`test/auth-test.mjs`).

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
mise run deploy                          # the Worker with its secrets from fnox, its D1 database, the migrations
mise run access:setup -- <email>...      # once: Cloudflare Access in front of it (see Access first), and its secrets
mise run access:token -- create live-test fnox    # once: the live test's machine token
mise run machine:enrol -- live-test 0000000000000001    # which device it posts for
mise run live-test                       # always after a deploy: every route, the TypeScript SDK and MCP, on Cloudflare
```

`deploy` sets the Worker's secrets from fnox every time: the two tokens ([Tokens](guides/tokens.md)) and Access's settings once `access:setup` has made them. Access and the machines' tokens: [Access](guides/access.md). The live test reports as a test machine, `0000000000000001` (host `live-test`), which stays in the list.

## 4. Read the fleet from anywhere

```sh
npx cf access login https://fleet-api.gedw99.workers.dev     # GitHub, in the browser
npx cf access curl https://fleet-api.gedw99.workers.dev/api/devices
```

On a phone: open the URL and log in with GitHub. From Go: [The Go SDK](guides/go-sdk.md). From an agent: the read routes are MCP tools at `/api/mcp`; the client logs in through Access's OAuth.

## 5. On GitHub

```sh
mise run cloudflare:secrets    # once: the deploy workflow's Cloudflare and Worker secrets, from fnox
mise run repo                  # the repo's description, topics, labels, issue forms, workflows, Pages, from charter.toml
mise run docs:pages            # once: GitHub Pages for docs/
```
