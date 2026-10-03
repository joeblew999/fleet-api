---
title: Routes
nav_order: 1
parent: Reference
---

# Routes

Every route sits behind Cloudflare Access; the scope column is what the Worker then requires ([Auth and authz](../concepts/auth.md)).

| Operation (MCP tool) | Route | Scope | Answer |
|---|---|---|---|
| `postDeviceReport` (not a tool) | `POST /api/devices/{id}/reports` | `devices:write`; a machine's token for its own `{id}` only | 201 `DevicePosted`: `id`, `received`, `duplicate`, `conditions` |
| `listDevices` | `GET /api/devices` | `devices:read` | `DeviceList`: `now`, `devices[]` (a `DeviceView` each), by id |
| `getDevice` | `GET /api/devices/{id}` | `devices:read` | `DeviceView`: `report` (as posted), `received`, `due`, `conditions`; 404 if it never reported |
| `listDeviceReports` | `GET /api/devices/{id}/reports?since=&limit=` | `devices:read` | `DeviceHistory`: `id`, `reports[]` (`report`, `received`), newest first; `since` in ms of receipt, `limit` 1 to 500 (50) |
| `deleteDevice` (not a tool) | `DELETE /api/devices/{id}` | `devices:forget` | `DeviceDeleted`: `id`, `reports` (how many went with it); 404 if it never reported. A machine that reports again comes back |
| `hello` | `GET /api/hello` | none | `message` |
| | `GET /.well-known/openid-configuration` | none | A redirect to the OpenID Connect issuer's; 404 when none is configured |
| | `GET /api/openapi.json` | none | The spec, with the request's origin as its server |
| | `POST /api/mcp` | each tool its operation's | The read operations and `hello` as MCP tools, run as the caller |

`{id}` must equal the report's `id`. No credentials, or ones that do not verify: 401. Credentials without the scope, or a machine posting for another device: 403. Errors are problem JSON (`application/problem+json`) with `status` and, for 422, `errors[].location`.

## Conditions

The Worker works these out from the newest report, when it arrived, and the time of the request.

| Code | Holds when |
|---|---|
| `stopped` | the newest report's reason is `stop` |
| `quiet` | a report was promised (`next_s`) and is more than three `next_s` late |
| `unminded` | `sleep.lid_action` is `nothing` and the keeper is not running, or the machine is quiet or stopped |
| `closed-on-battery` | lid closed, on battery, and still reporting |
| `battery-low` | on battery, under 20 percent |
| `rig-unready` | `rig` is ok and its tools are not installed, its configuration is not applied, or Claude is not logged in |

## Storage (D1, `migrations/`)

| Table | Holds |
|---|---|
| `devices` | One row per machine: its newest report by `ts`, as posted, with `received`, `due`, `reason`, `next_s`, `name`, `os` |
| `device_reports` | Every report, keyed `(id, ts)`, kept 7 days from receipt; older ones are deleted when a report is posted |
