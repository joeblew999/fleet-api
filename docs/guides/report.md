---
title: The report
nav_order: 1
parent: Guides
---

# The report: what a machine says about itself

A machine posts one JSON object, a `DeviceReport` (schema 1), to `POST /api/devices/{id}/reports` with its own Access service token ([Access](access.md)). The schema, with every field's rule, is in `api/device.go` and `fern/openapi.json`; a valid one is `api/example_report.json`.

## The parts

| Part | Fields | Answers |
|---|---|---|
| envelope | `schema`, `id`, `ts`, `reason` (start, change, interval, stop, once), `next_s` | always |
| `tool` | `version`, `command`; optional `name` | always |
| `host` | `name`, `os` (darwin, linux, windows), `arch`; optional `os_name`, `os_version`, `model`, `guest`, `boot` | always |
| `cpu` | `count`, optional `load1` | ok, unknown |
| `memory` | `total`, `available` | ok, unknown |
| `disks[]` | `roles` (system, data), `path`, `fs`, `total`, `free` | ok, unknown, each |
| `power` | `source` (ac, battery, ups) | ok, unknown |
| `battery` | `count`, `percent`, `state`; optional `health`, `remaining_s` | ok, none, unknown |
| `lid` | `closed` | ok, none, unknown |
| `sleep` | `idle_s`, `inhibited`; optional `display_s`, `lid_action`, `inhibitors[]` | ok, none, unknown |
| `keeper` | `running`, `idle`, `lid` | ok, none, unknown |
| `rig`, optional | `tools_installed`, `config_applied`, `logged_in`, `session_running`; optional `commit`, `claude_version`, `work_dir`, `login` | ok, none, unknown |
| `rig.login`, optional | `logged_in`; optional `auth_method`, `refresh_expires` or `refresh_expires_why` | ok, unknown |
| `claims`, optional | `slots`; optional `held[]`: `id`, `caller`, `job`, `since`, optional `until` | ok, none, unknown |
| `vms`, optional | `manager_running`; optional `manager`, `list[]`: `name`, `state`, optional `os`, `owner`, `keep_running`, `keeper_starts` | ok, none, unknown |

`rig` is what claude-rig's `doctor --json` reads, and `rig.login` what `claude auth status` and the stored login say (never a token, an email address or an organisation); `claims` is who holds the machine, as its claims folder says; `vms` is the virtual machines on it, as the VM tool's keeper (`irgo-winvm keeper`) sees them, with the manager closed meaning every one is stopped. A tool that does not know them leaves them out.

## The rules

- **Three answers per section.** `status` is `ok` (the values are there), `none` (this machine has no such thing) or `unknown` (it could not be read, and `why` says why). A section that is not `ok` carries no values, so a zero is always a measured zero.
- **The machine says when it reports next** (`next_s`; 0 for `stop` and `once`). The Worker judges lateness by when it received the report, never by the machine's clock.
- **Stored as posted.** A newer tool's fields come back unchanged; every object allows fields it does not know.
- **Nothing that identifies a person or a network.** No serial number, MAC or IP address, or user name. A home directory is written `~` (`/Users/<name>`, `/home/<name>` and `C:\Users\<name>` are refused); a claim's `caller` and a VM's `owner` name an agent, session or repository and have no `@`. The `id` is 16 random hex digits made once per machine.
- **Units:** bytes as integers, times as Unix milliseconds, durations in seconds with `_s`, percent 0 to 100, snake_case.
- **Bounds:** 16 KiB a report (413 over it), 200 bytes a string, 8 disks, 8 inhibitors, 64 claims, 32 VMs.
- **Schema changes:** a new optional field or section keeps `schema` 1; a changed meaning or a removed field makes it 2, and this Worker refuses 2.

A report that breaks a rule is refused with 422 and the field's location (`body.battery.percent`). The same `id` and `ts` again is a duplicate and changes nothing, so a machine can resend what it could not deliver.

## Post one from a machine

One program posts for a machine, so a machine has one `id`: claude-rig, which reads the machine and takes the `vms` section from the VM tool. It posts through the TypeScript SDK a release attaches (`fleet-api-sdk-typescript.tar.gz`, sources), run with [bun](https://bun.sh); nothing is written by hand against the routes:

```ts
import { FleetClient, serialization } from "./fleet-api-sdk-typescript/index.ts";

const checked = serialization.DeviceReport.parse(report);   // the report as JSON, against the schema
if (!checked.ok) throw new Error(checked.errors.map(e => `${e.path.join(".")}: ${e.message}`).join("; "));
// The machine's service token; both headers by hand until fern-api/fern#17775 is fixed.
const { FLEET_API_ACCESS_CLIENT_ID: id, FLEET_API_ACCESS_CLIENT_SECRET: secret } = process.env;
const client = new FleetClient({ baseUrl: "https://fleet-api.gedw99.workers.dev", auth: false, headers: { "CF-Access-Client-Id": id, "CF-Access-Client-Secret": secret } });
await client.devices.report({ id: checked.value.id, body: checked.value });
```

`parse` checks the types, the enums and the required fields; the Worker checks the rest (bounds, the rules above) and answers 422. A person forgets a machine that is gone, or an id no longer used: `client.devices.delete({ id })` (`devices:forget`; a machine's token may not).
