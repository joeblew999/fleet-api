---
title: Tokens
nav_order: 2
parent: Guides
---

# Tokens: who may write, who may read

Every route but `/api/hello` and the specs needs `Authorization: Bearer <token>`.

| Token | Worker secret | fnox name, keychain item and variable the scripts read | Can |
|---|---|---|---|
| write | `WRITE_TOKEN` | `FLEET_API_WRITE_TOKEN` | post reports, forget a machine, and read |
| read | `READ_TOKEN` | `FLEET_API_READ_TOKEN` | read |

The values live in the macOS keychain of the owner's Mac and in the Worker's secrets, nowhere else: `fnox.toml` only names them, and no task prints them. An unset secret matches no token. Outside the Worker a token has one name, prefixed with the repo's, the rule for every repo of the owner's ([claude-rig's Secrets page](https://github.com/joeblew999/claude-rig/blob/main/docs/concepts/secrets.md)); `scripts/put-tokens.mjs` stores each as the Worker's shorter name.

`mise.toml` sets `FNOX_PROFILE=fleet-api` and `FNOX_NO_DEFAULTS=true`, so in this folder (with mise active) fnox sees only what `fnox.toml` declares: the two tokens and the Cloudflare credentials, not the rest of the global fnox config.

## Make them, or rotate them

```sh
openssl rand -hex 32 | tr -d '\n' | fnox set FLEET_API_READ_TOKEN -p keychain
openssl rand -hex 32 | tr -d '\n' | fnox set FLEET_API_WRITE_TOKEN -p keychain
mise run build && mise run tokens:put    # sets both secrets on the Worker; for a Worker not yet deployed, deploys it with them
```

After a rotation every machine needs the new write token.

## Give a machine the write token

The machine keeps it, readable by its user alone; it is never in a repo, a captured config or a VM image. claude-rig passes it when it rigs a machine (not built yet).

## Use one

```sh
fnox exec -- sh -c 'curl -s -H "authorization: Bearer $FLEET_API_READ_TOKEN" https://fleet-api.gedw99.workers.dev/api/devices'
```

The local checks and `mise run run` use throwaway tokens set in `mise.toml`: `local-read-token`, `local-write-token`, as `READ_TOKEN` and `WRITE_TOKEN` for the server and `FLEET_API_READ_TOKEN` and `FLEET_API_WRITE_TOKEN` for the test clients.
