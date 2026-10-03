---
title: Access
nav_order: 2
parent: Guides
---

# Access: protect the Worker, give a machine its token, take it back

Cloudflare Access stands in front of `https://fleet-api.gedw99.workers.dev`: people log in with GitHub, machines show their own service token. Why and how it fits: [Auth and authz](../concepts/auth.md).

Every task here runs under `fnox exec` and prints no secret. fnox needs `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CF_ACCESS_TEAM_DOMAIN`, `CF_ACCESS_GITHUB_IDP_ID` and `FLEET_API_ACCESS_EMAILS` (who may log in, comma-separated).

## Protect the Worker

```sh
mise run migrate          # the machines table
mise run access:setup     # the Access application, its policies, the Worker's ACCESS_TEAM_DOMAIN and ACCESS_AUD
mise run deploy
```

`access:setup` is idempotent; run it again after changing `FLEET_API_ACCESS_EMAILS`. It writes the application's id and AUD tag to fnox (`FLEET_API_ACCESS_APP_ID`, `FLEET_API_ACCESS_AUD`) and turns on Access's OAuth for MCP clients.

| Policy | Decision | Lets in |
|---|---|---|
| `fleet-api machines` | Service Auth | each machine's service token, by id |
| `fleet-api developers` | Allow | the addresses in `FLEET_API_ACCESS_EMAILS`, logged in with GitHub |

Without credentials a browser is sent to the GitHub login (302), a program gets 302 or 401: the Worker never sees the request.

## Give a machine its token

```sh
mise run access:token -- create <machine> <device id> <file>
```

- `<machine>`: a name, lower-case letters, digits and dashes; the token is `fleet-api-<machine>`.
- `<device id>`: the 16 hex digits the machine reports as (claude-rig keeps it in `~/.config/claude-rig/device-id`). The token posts for that device only; another id is 403.
- `<file>`: where its Client ID and Secret go, readable by its owner alone (`{"client_id": ..., "client_secret": ...}`). Cloudflare shows the secret once. `fnox` instead of a file stores them as `FLEET_API_ACCESS_CLIENT_ID` and `FLEET_API_ACCESS_CLIENT_SECRET`: the live test's machine, `live-test`, for `0000000000000001`.

A token lasts a year (`8760h`). The machine sends both headers on every request:

```sh
curl -H "CF-Access-Client-Id: $ID" -H "CF-Access-Client-Secret: $SECRET" https://fleet-api.gedw99.workers.dev/api/devices
```

## List and revoke

```sh
mise run access:token -- list                 # name, device, expiry; no secret
mise run access:token -- revoke <machine>     # Access refuses it from then on
```

Revoking deletes the token, its row in the `machines` table and its place in the policy. The other machines are untouched.

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
