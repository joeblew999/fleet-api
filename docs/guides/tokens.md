---
title: Tokens
nav_order: 4
parent: Guides
---

# Tokens: the read and write tokens, for one release

Machines and people now prove who they are through Cloudflare Access ([Access](access.md)). The two bearer tokens remain for one release, then go: behind Access a bearer token alone is refused at the edge, so they work only locally (`mise run run`, the checks) or beside Access credentials.

| Token | Worker secret | fnox name, keychain item and variable the scripts read | Can |
|---|---|---|---|
| write | `WRITE_TOKEN` | `FLEET_API_WRITE_TOKEN` | `devices:read`, `devices:write` (any device), `devices:forget` |
| read | `READ_TOKEN` | `FLEET_API_READ_TOKEN` | `devices:read` |

The values live in the macOS keychain of the owner's Mac and in the Worker's secrets, nowhere else: `fnox.toml` only names them, and no task prints them. An unset secret matches no token. Outside the Worker a token has one name, prefixed with the repo's, the rule for every repo of the owner's ([claude-rig's Secrets page](https://github.com/joeblew999/claude-rig/blob/main/docs/concepts/secrets.md)); `scripts/put-tokens.mjs` stores each as the Worker's shorter name.

`mise.toml` sets `FNOX_PROFILE=fleet-api` and `FNOX_NO_DEFAULTS=true`, so in this folder (with mise active) fnox sees only what `fnox.toml` declares: this repo's secrets and the Cloudflare credentials, not the rest of the global fnox config.

## Make them, or rotate them

```sh
openssl rand -hex 32 | tr -d '\n' | fnox set FLEET_API_READ_TOKEN -p keychain
openssl rand -hex 32 | tr -d '\n' | fnox set FLEET_API_WRITE_TOKEN -p keychain
mise run build && mise run tokens:put    # sets both secrets on the Worker; for a Worker not yet deployed, deploys it with them
```

A machine gets its own service token instead: [Access](access.md#give-a-machine-its-token).

## Use one

```sh
mise run run    # then:
curl -s -H "authorization: Bearer local-read-token" localhost:5174/api/devices
```

The local checks and `mise run run` use throwaway tokens set in `mise.toml`: `local-read-token`, `local-write-token`, as `READ_TOKEN` and `WRITE_TOKEN` for the server and `FLEET_API_READ_TOKEN` and `FLEET_API_WRITE_TOKEN` for the test clients.
