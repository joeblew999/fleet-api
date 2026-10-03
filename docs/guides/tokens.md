---
title: Tokens
nav_order: 4
parent: Guides
---

# Tokens: the read and write tokens, for one release

Machines and people now prove who they are through Cloudflare Access ([Access](access.md)). The two bearer tokens remain for one release, then go: behind Access a bearer token alone is refused at the edge, so they work only locally (`mise run run`, the checks) or beside Access credentials.

| Token | Worker secret and fnox name | Keychain item | Can |
|---|---|---|---|
| write | `WRITE_TOKEN` | `FLEET_API_WRITE_TOKEN` | `devices:read`, `devices:write` (any device), `devices:forget` |
| read | `READ_TOKEN` | `FLEET_API_READ_TOKEN` | `devices:read` |

The values live in the macOS keychain of the owner's Mac and in the Worker's secrets, nowhere else: `fnox.toml` only names them, and no task prints them. An unset secret matches no token. The fnox name is the Worker's, as charter's `deploy` reads it (`WORKER_SECRETS` in `mise.toml`); the keychain item is prefixed with the repo's name, the rule for every repo of the owner's ([claude-rig's Secrets page](https://github.com/joeblew999/claude-rig/blob/main/docs/concepts/secrets.md)). Every `mise run deploy` sets both on the Worker, and creates a Worker that does not exist yet with them.

`mise.toml` sets `FNOX_PROFILE=fleet-api` and `FNOX_NO_DEFAULTS=true`, so in this folder (with mise active) fnox sees only what `fnox.toml` declares: this repo's secrets and the Cloudflare credentials, not the rest of the global fnox config.

## Make them, or rotate them

```sh
openssl rand -hex 32 | tr -d '\n' | fnox set READ_TOKEN -p keychain -k FLEET_API_READ_TOKEN
openssl rand -hex 32 | tr -d '\n' | fnox set WRITE_TOKEN -p keychain -k FLEET_API_WRITE_TOKEN
mise run deploy    # sets both secrets on the Worker
```

A machine gets its own service token instead: [Access](access.md#give-a-machine-its-token).

## Use one

```sh
mise run run    # then:
curl -s -H "authorization: Bearer local-read-token" localhost:5174/api/devices
```

The local checks and `mise run run` use throwaway tokens set in `mise.toml` (and in charter's shared `run` and `dev`): `local-read-token`, `local-write-token`, as `READ_TOKEN` and `WRITE_TOKEN` for the server and the test clients alike.
