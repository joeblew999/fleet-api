---
title: The Go SDK
nav_order: 3
parent: Guides
---

# The Go SDK: report and read from another repo

Fern generates it from the contract; `mise run sdk:publish` commits it to `sdk/go/`. Never edit it.

```sh
go get github.com/joeblew999/fleet-api/sdk/go@main
```

```go
import (
	fleet "github.com/joeblew999/fleet-api/sdk/go"
	"github.com/joeblew999/fleet-api/sdk/go/client"
	"github.com/joeblew999/fleet-api/sdk/go/option"
)

machine := client.NewClient(option.WithBaseURL("https://fleet-api.gedw99.workers.dev"), option.WithToken(writeToken))
posted, err := machine.Devices.Report(ctx, &fleet.ReportDevicesRequest{ID: report.ID, Body: &report})

reader := client.NewClient(option.WithBaseURL("https://fleet-api.gedw99.workers.dev"), option.WithToken(readToken))
list, err := reader.Devices.List(ctx)                                        // every machine
view, err := reader.Devices.Get(ctx, &fleet.GetDevicesRequest{ID: id})       // one, with its conditions
history, err := reader.Devices.History(ctx, &fleet.HistoryDevicesRequest{ID: id})
```

Enums are Go types, optional values pointers, a newer Worker's fields are kept in `ExtraProperties`. Errors are typed by status: `*fleet.UnauthorizedError` (401), `*fleet.UnprocessableEntityError` (422, with the locations), `*fleet.NotFoundError` (404), `*fleet.ContentTooLargeError` (413).

After a contract change: `mise run spec`, then `mise run sdk:publish`, and commit `sdk/go/` with it. The TypeScript SDK: `mise run sdk:gen typescript-dist` (`FleetClient`, option `token`).
