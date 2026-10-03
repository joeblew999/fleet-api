---
title: Benchmarks
nav_order: 2
parent: How to help
---

# Benchmarks: what was run on Cloudflare, and what it cost

The only page with the numbers. Each says where and when it was measured.

## Deployed and live-tested (2026-10-03)

- `mise run deploy` to `https://fleet-api.gedw99.workers.dev`, the account on Workers Paid. A new Worker needs its secrets on the first deploy: `mise run tokens:put` does that deploy.
- `mise run live-test`: 42 checks pass, every route raw (18), through the generated TypeScript SDK (7) and over MCP in both protocol eras (17).
- A Go program using the committed SDK (`sdk/go/`) posted a report with the write token, listed the machines with the read token, and got `*fleet.UnauthorizedError` posting with the read token.

## A real machine's report (2026-10-03)

The owner's Mac ran claude-rig's `doctor --json` (claude-rig `d7b2e7e`); it was wrapped into a report by hand: `host` and `rig` from doctor (`work_dir` written `~/work`), `claims` from claude-rig's claims list (1 slot, none held), the hardware sections `unknown` ("claude-rig doctor does not read this"). Posted with the write token: 201, no conditions, 0.23 s wall from the Mac. Read back with the read token through `getDevice`, `listDevices` and `listDeviceReports`: the report identical to what was posted.

## CPU per request

`mise run bench` (Cloudflare's CPU time, from Workers Logs), 2026-10-03, 20 warm requests each, a store of 2 to 3 machines:

| Operation | CPU median | CPU p99 |
|---|---|---|
| `GET /api/hello` | 1 ms | 1 ms |
| `GET /api/devices` | 3 to 4 ms | 9 to 14 ms |
| `GET /api/devices/{id}` | 3 ms | 15 ms |
| `GET /api/devices/{id}/reports` | 3 to 4 ms | 6 ms |
| `POST /api/devices/{id}/reports` (3 D1 statements) | 4 ms | 10 ms |

Every request succeeded. A request that starts a Go runtime is the p99; most are under Workers Free's 10 ms, some are not, so this Worker is sized for Workers Paid.

## Found on the way

- Under TinyGo, Huma panics on an integer tag over 2^31 (`example:"1790842406355"` on an `int64` query parameter: "invalid integer tag value"), because `int` is 32-bit there. `go test` passes; only `mise run test:workerd` and Cloudflare show it. The contract's millisecond examples are small for that reason.
