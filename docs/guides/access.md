---
title: Access
nav_order: 2
parent: Guides
---

# Access: protect the Worker, give a machine its token, take it back

Cloudflare Access stands in front of `https://fleet-api.gedw99.workers.dev`: people log in with GitHub, machines show their own service token. Why and how it fits: [Auth and authz](../concepts/auth.md).

The Access tasks are charter's (`charter access`, the shared tasks `access:setup`, `access:token`, `access:delete`); which device a machine's token posts for is fleet-api's own (`machine:enrol`, `machine:list`, `machine:forget`, `scripts/machines.mjs`). Every one prints no secret. They need `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CF_ACCESS_TEAM_DOMAIN` and `CF_ACCESS_GITHUB_IDP_ID` in fnox.

## Protect the Worker

```sh
mise run deploy                               # the Worker, and the machines table
mise run access:setup -- <email>...           # the Access application, its policies, the Worker's ACCESS_TEAM_DOMAIN and ACCESS_AUD
```

`access:setup` is idempotent; run it again with the new list after changing who may log in (without emails it keeps the ones the application has). It puts `ACCESS_TEAM_DOMAIN`, `ACCESS_AUD` and `ACCESS_APP_ID` in fnox, and every later `mise run deploy` sets the first two on the Worker again (`WORKER_OPTIONAL_SECRETS` in `mise.toml`).

| Policy | Decision | Lets in |
|---|---|---|
| `fleet-api machines` | Service Auth | each service token named `fleet-api:<machine>`, by id |
| `fleet-api people` | Allow | the addresses given to `access:setup`, logged in with GitHub |

Without credentials a browser is sent to the GitHub login (302), a program gets 302 or 401: the Worker never sees the request.

Access's OAuth for MCP clients (the application's `oauth_configuration`, with dynamic client registration for `https://claude.ai/api/mcp/auth_callback`) is set in the Zero Trust dashboard, not by `access:setup`; charter's `access` keeps it, and every other setting it does not set itself, when it writes the application.

## Give a machine its token

```sh
mise run access:token -- create <machine> <file>
mise run machine:enrol -- <machine> <device id>
```

- `<machine>`: a name, lower-case letters, digits and dashes; the token is `fleet-api:<machine>`, charter's name for it, which is what puts it in the `fleet-api machines` policy.
- `<file>`: where its Client ID and Secret go, readable by its owner alone (`{"client_id": ..., "client_secret": ...}`). Cloudflare shows the secret once. `fnox` instead of a file stores them as `FLEET_API_ACCESS_CLIENT_ID` and `FLEET_API_ACCESS_CLIENT_SECRET`: the live test's machine, `live-test`, for `0000000000000001`.
- `<device id>`: the 16 hex digits the machine reports as (claude-rig keeps it in `~/.config/claude-rig/device-id`). The Worker knows the token by its Client ID (the `machines` table); it posts for that device only, another id is 403, and a token that was never enrolled reads but may not post.

A token lasts a year (`8760h`). The machine sends both headers on every request:

```sh
curl -H "CF-Access-Client-Id: $ID" -H "CF-Access-Client-Secret: $SECRET" https://fleet-api.gedw99.workers.dev/api/devices
```

## List and revoke

```sh
mise run machine:list                         # each token, the device it posts for, its expiry; no secret
mise run access:token -- revoke <machine>     # Access refuses it from then on
mise run machine:forget -- <machine>          # and the Worker no longer ties it to a device
```

Revoking deletes the token and its place in the policy. The other machines are untouched.

## From the SDKs

```go
// Go: reads FLEET_API_ACCESS_CLIENT_ID and FLEET_API_ACCESS_CLIENT_SECRET, or:
c := client.NewClient(option.WithBaseURL(url), option.WithAccessClientID(id), option.WithAccessClientSecret(secret))
```

```ts
// TypeScript: both headers by hand until fern-api/fern#17775 is fixed.
const client = new FleetClient({ baseUrl, auth: false, headers: { "CF-Access-Client-Id": id, "CF-Access-Client-Secret": secret } });
```

## As a person

A browser: open the URL, log in with GitHub. A terminal: `npx cf access login https://fleet-api.gedw99.workers.dev`, then `npx cf access curl <url>`. An MCP client: add `https://fleet-api.gedw99.workers.dev/api/mcp`; it is sent to Access's OAuth and the GitHub login.
