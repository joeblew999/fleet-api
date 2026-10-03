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

// A machine's Access service token: from FLEET_API_ACCESS_CLIENT_ID and _SECRET, or the options.
machine := client.NewClient(option.WithBaseURL("https://fleet-api.gedw99.workers.dev"),
	option.WithAccessClientID(id), option.WithAccessClientSecret(secret))
posted, err := machine.Devices.Report(ctx, &fleet.ReportDevicesRequest{ID: report.ID, Body: &report})

reader := machine                                                            // a machine reads too
list, err := reader.Devices.List(ctx)                                        // every machine
view, err := reader.Devices.Get(ctx, &fleet.GetDevicesRequest{ID: id})       // one, with its conditions
history, err := reader.Devices.History(ctx, &fleet.HistoryDevicesRequest{ID: id})

// A person: the JWT that `npx cf access token <url>` prints, after `npx cf access login <url>`.
person := client.NewClient(option.WithBaseURL("https://fleet-api.gedw99.workers.dev"), option.WithHTTPHeader(http.Header{"Cf-Access-Token": {jwt}}))
gone, err := person.Devices.Delete(ctx, &fleet.DeleteDevicesRequest{ID: id})         // forget it: a person's (devices:forget)
```

Enums are Go types, optional values pointers, a newer Worker's fields are kept in `ExtraProperties`. A person or an app with an OpenID Connect token: `option.WithAccessToken(token)`; the bearer tokens, for one release: `option.WithToken(token)`. Errors are typed by status: `*fleet.UnauthorizedError` (401), `*fleet.ForbiddenError` (403: no scope, or another machine's id), `*fleet.UnprocessableEntityError` (422, with the locations), `*fleet.NotFoundError` (404), `*fleet.ContentTooLargeError` (413).

After a contract change: `mise run spec`, then `mise run sdk:publish`, and commit `sdk/go/` with it. The TypeScript SDK: `mise run sdk:gen typescript-dist` (`FleetClient`; its service-token headers by hand, [Access](access.md#from-the-sdks); `serialization.DeviceReport.parse` checks a report as JSON, [Post one from a machine](report.md#post-one-from-a-machine)).
