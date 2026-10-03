---
title: Routes
nav_order: 1
parent: Reference
---

# Routes

| Operation (MCP tool) | Route | Token | Answer |
|---|---|---|---|
| `postDeviceReport` (not a tool) | `POST /api/devices/{id}/reports` | write | 201 `DevicePosted`: `id`, `received`, `duplicate`, `conditions` |
| `listDevices` | `GET /api/devices` | read or write | `DeviceList`: `now`, `devices[]` (a `DeviceView` each), by id |
| `getDevice` | `GET /api/devices/{id}` | read or write | `DeviceView`: `report` (as posted), `received`, `due`, `conditions`; 404 if it never reported |
| `listDeviceReports` | `GET /api/devices/{id}/reports?since=&limit=` | read or write | `DeviceHistory`: `id`, `reports[]` (`report`, `received`), newest first; `since` in ms of receipt, `limit` 1 to 500 (50) |
| `hello` | `GET /api/hello` | none | `message` |
| | `GET /api/openapi.json` | none | The spec, with the request's origin as its server |
| | `POST /api/mcp` | read, as `Authorization` | The read operations and `hello` as MCP tools |

`{id}` must equal the report's `id`. Errors are problem JSON (`application/problem+json`) with `status` and, for 422, `errors[].location`.

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
