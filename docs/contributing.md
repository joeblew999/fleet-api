---
title: How to help
nav_order: 6
has_children: true
---

# How to help

[The rules](rules.md) are short and binding.

## Set up

```sh
git clone https://github.com/joeblew999/fleet-api && cd fleet-api
mise install && mise run setup
mise run check                    # everything local: no Cloudflare account needed
```

`mise run doctor` says what is missing.

## Change the report

1. Change `api/device.go` (a tag for what a schema can say, `Validate` for the rest) and `api/example_report.json` if the example should show it.
2. `mise run spec`, then `mise run check`.
3. `mise run sdk:publish` and commit `sdk/go/` with the change.
4. Update [The report](guides/report.md). A new optional field or section keeps `schema` 1.

## Report a bug

Use the GitHub issue forms. An agent prints the same form with `charter issue bug > body.md`.

## Before a pull request

`mise run check` passes, `docs/` says what changed, and after a deploy `mise run live-test` passes.
