> **Generated code.** Fern generated this Go client from the API's contract (the specs in `fern/`). Don't edit it: change the contract, then `mise run sdk:publish` writes this folder again.

# FleetApi Go Library

[![fern shield](https://img.shields.io/badge/%F0%9F%8C%BF-Built%20with%20Fern-brightgreen)](https://buildwithfern.com?utm_source=github&utm_medium=github&utm_campaign=readme&utm_source=FleetApi%2FGo)

The FleetApi Go library provides convenient access to the FleetApi APIs from Go.

## Table of Contents

- [Reference](#reference)
- [Usage](#usage)
- [Environments](#environments)
- [Errors](#errors)
- [Request Options](#request-options)
- [Advanced](#advanced)
  - [Response Headers](#response-headers)
  - [Retries](#retries)
  - [Timeouts](#timeouts)
  - [Explicit Null](#explicit-null)
- [Contributing](#contributing)

## Reference

A full reference for this library is available [here](./reference.md).

## Usage

Instantiate and use the client with the following:

```go
package example

import (
    context "context"

    fleet "github.com/joeblew999/fleet-api/sdk/go"
    client "github.com/joeblew999/fleet-api/sdk/go/client"
    option "github.com/joeblew999/fleet-api/sdk/go/option"
)

func do() {
    client := client.NewClient(
        option.WithAccessClientID(
            "<value>",
        ),
    )
    request := &fleet.ReportDevicesRequest{
        ID: "3f9a1c0b7d2e4a65",
        Body: &fleet.DeviceReport{
            Battery: &fleet.DeviceBattery{
                Count: fleet.Int(
                    1,
                ),
                Percent: fleet.Float64(
                    100,
                ),
                State: fleet.DeviceBatteryStateFull.Ptr(),
                Status: fleet.DeviceBatteryStatusOk,
            },
            Claims: &fleet.DeviceClaims{
                Held: []*fleet.DeviceClaim{
                    &fleet.DeviceClaim{
                        Caller: "lead-agent",
                        ID: "20261003-102233-ab12c",
                        Job: "run the tests on macOS",
                        Since: int64(1790842300000),
                        Until: fleet.Int64(
                            int64(1790843200000),
                        ),
                    },
                },
                Slots: fleet.Int(
                    2,
                ),
                Status: fleet.DeviceClaimsStatusOk,
            },
            CPU: &fleet.DeviceCPU{
                Count: fleet.Int(
                    12,
                ),
                Load1: fleet.Float64(
                    2.73,
                ),
                Status: fleet.DeviceCPUStatusOk,
            },
            Disks: []*fleet.DeviceDisk{
                &fleet.DeviceDisk{
                    Free: fleet.Int64(
                        int64(16648376320),
                    ),
                    Fs: fleet.String(
                        "apfs",
                    ),
                    Path: "/System/Volumes/Data",
                    Roles: []fleet.DeviceDiskRolesItem{
                        fleet.DeviceDiskRolesItemSystem,
                        fleet.DeviceDiskRolesItemData,
                    },
                    Status: fleet.DeviceDiskStatusOk,
                    Total: fleet.Int64(
                        int64(494384795648),
                    ),
                },
            },
            Host: &fleet.DeviceHost{
                Arch: "arm64",
                Boot: fleet.Int64(
                    int64(1790690961000),
                ),
                Guest: fleet.Bool(
                    false,
                ),
                Model: fleet.String(
                    "Mac14,10",
                ),
                Name: "studio-1",
                Os: fleet.DeviceHostOsDarwin,
                OsName: fleet.String(
                    "macOS",
                ),
                OsVersion: fleet.String(
                    "27.0.1",
                ),
            },
            ID: "3f9a1c0b7d2e4a65",
            Keeper: &fleet.DeviceKeeper{
                Status: fleet.DeviceKeeperStatusNone,
            },
            Lid: &fleet.DeviceLid{
                Closed: fleet.Bool(
                    false,
                ),
                Status: fleet.DeviceLidStatusOk,
            },
            Memory: &fleet.DeviceMemory{
                Available: fleet.Int64(
                    int64(3869261824),
                ),
                Status: fleet.DeviceMemoryStatusOk,
                Total: fleet.Int64(
                    int64(17179869184),
                ),
            },
            NextS: int64(300),
            Power: &fleet.DevicePower{
                Source: fleet.DevicePowerSourceAc.Ptr(),
                Status: fleet.DevicePowerStatusOk,
            },
            Reason: fleet.DeviceReportReasonInterval,
            Rig: &fleet.DeviceRig{
                ClaudeVersion: fleet.String(
                    "2.1.0 (Claude Code)",
                ),
                Commit: fleet.String(
                    "4fd9d8e",
                ),
                ConfigApplied: fleet.Bool(
                    true,
                ),
                LoggedIn: fleet.Bool(
                    true,
                ),
                Login: &fleet.DeviceRigLogin{
                    AuthMethod: fleet.String(
                        "claude.ai",
                    ),
                    LoggedIn: fleet.Bool(
                        true,
                    ),
                    RefreshExpires: fleet.Int64(
                        int64(1798618406000),
                    ),
                    Status: fleet.DeviceRigLoginStatusOk,
                },
                SessionRunning: fleet.Bool(
                    true,
                ),
                Status: fleet.DeviceRigStatusOk,
                ToolsInstalled: fleet.Bool(
                    true,
                ),
                WorkDir: fleet.String(
                    "~/claude-work",
                ),
            },
            Schema: 1,
            Sleep: &fleet.DeviceSleep{
                DisplayS: fleet.Int64(
                    int64(1800),
                ),
                IdleS: fleet.Int64(
                    int64(0),
                ),
                Inhibited: fleet.Bool(
                    true,
                ),
                Inhibitors: []string{
                    "caffeinate",
                },
                LidAction: fleet.DeviceSleepLidActionSleep.Ptr(),
                Status: fleet.DeviceSleepStatusOk,
            },
            Tool: &fleet.DeviceTool{
                Command: "doctor",
                Name: fleet.String(
                    "claude-rig",
                ),
                Version: "4fd9d8e",
            },
            Ts: int64(1790842406355),
            Vms: &fleet.DeviceVMs{
                List: []*fleet.DeviceVM{
                    &fleet.DeviceVM{
                        KeepRunning: fleet.Bool(
                            true,
                        ),
                        KeeperStarts: fleet.Int(
                            1,
                        ),
                        Name: "claude-rig-linux",
                        Os: fleet.DeviceVMOsLinux.Ptr(),
                        Owner: fleet.String(
                            "claude-rig",
                        ),
                        State: "started",
                    },
                    &fleet.DeviceVM{
                        Name: "irgo-golden",
                        State: "stopped",
                    },
                },
                Manager: fleet.String(
                    "utm",
                ),
                ManagerRunning: fleet.Bool(
                    true,
                ),
                Status: fleet.DeviceVMsStatusOk,
            },
        },
    }
    client.Devices.Report(
        context.TODO(),
        request,
    )
}
```

## Environments

You can choose between different environments by using the `option.WithBaseURL` option. You can configure any arbitrary base
URL, which is particularly useful in test environments.

```go
client := client.NewClient(
    option.WithBaseURL(fleet.Environments.Default),
)
```

## Errors

Structured error types are returned from API calls that return non-success status codes. These errors are compatible
with the `errors.Is` and `errors.As` APIs, so you can access the error like so:

```go
response, err := client.Devices.Report(...)
if err != nil {
    var apiError *core.APIError
    if errors.As(err, &apiError) {
        // Do something with the API error ...
    }
    return err
}
```

## Request Options

A variety of request options are included to adapt the behavior of the library, which includes configuring
authorization tokens, or providing your own instrumented `*http.Client`.

These request options can either be
specified on the client so that they're applied on every request, or for an individual request, like so:

> Providing your own `*http.Client` is recommended. Otherwise, the `http.DefaultClient` will be used,
> and your client will wait indefinitely for a response (unless the per-request, context-based timeout
> is used).

```go
// Specify default options applied on every request.
client := client.NewClient(
    option.WithAccessClientID("<YOUR_API_KEY>"),
    option.WithAccessClientSecret("<YOUR_API_KEY>"),
    option.WithAccessToken("<YOUR_API_KEY>"),
    option.WithToken("<YOUR_API_KEY>"),
    option.WithHTTPClient(
        &http.Client{
            Timeout: 5 * time.Second,
        },
    ),
)

// Specify options for an individual request.
response, err := client.Devices.Report(
    ...,
    option.WithAccessClientID("<YOUR_API_KEY>"),
)
```

When credentials are not explicitly provided, the client reads them from the
following environment variables:

- `FLEET_API_ACCESS_CLIENT_ID`
- `FLEET_API_ACCESS_CLIENT_SECRET`
- `FLEET_API_ACCESS_TOKEN`

## Advanced

### Response Headers

You can access the raw HTTP response data by using the `WithRawResponse` field on the client. This is useful
when you need to examine the response headers received from the API call. (When the endpoint is paginated,
the raw HTTP response data will be included automatically in the Page response object.)

```go
response, err := client.Devices.WithRawResponse.Report(...)
if err != nil {
    return err
}
fmt.Printf("Got response headers: %v", response.Header)
fmt.Printf("Got status code: %d", response.StatusCode)
```

### Retries

The SDK is instrumented with automatic retries with exponential backoff. A request will be retried as long
as the request is deemed retryable and the number of retry attempts has not grown larger than the configured
retry limit (default: 2).

Which status codes are retried depends on the `retryStatusCodes` generator configuration:

**`legacy`** (current default): retries on
- [408](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/408) (Timeout)
- [429](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/429) (Too Many Requests)
- [5XX](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status#server_error_responses) (All server errors, including 500)

**`recommended`**: retries on
- [408](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/408) (Timeout)
- [429](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/429) (Too Many Requests)
- [502](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/502) (Bad Gateway)
- [503](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503) (Service Unavailable)
- [504](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/504) (Gateway Timeout)

If the `Retry-After` header is present in the response, the SDK will prioritize respecting its value exactly
over the default exponential backoff.

Use the `option.WithMaxAttempts` option to configure this behavior for the entire client or an individual request:

```go
client := client.NewClient(
    option.WithMaxAttempts(1),
)

response, err := client.Devices.Report(
    ...,
    option.WithMaxAttempts(1),
)
```

### Timeouts

Setting a timeout for each individual request is as simple as using the standard context library. Setting a one second timeout for an individual API call looks like the following:

```go
ctx, cancel := context.WithTimeout(ctx, time.Second)
defer cancel()

response, err := client.Devices.Report(ctx, ...)
```

### Explicit Null

If you want to send the explicit `null` JSON value through an optional parameter, you can use the setters\
that come with every object. Calling a setter method for a property will flip a bit in the `explicitFields`
bitfield for that setter's object; during serialization, any property with a flipped bit will have its
omittable status stripped, so zero or `nil` values will be sent explicitly rather than omitted altogether:

```go
type ExampleRequest struct {
    // An optional string parameter.
    Name *string `json:"name,omitempty" url:"-"`

    // Private bitmask of fields set to an explicit value and therefore not to be omitted
    explicitFields *big.Int `json:"-" url:"-"`
}

request := &ExampleRequest{}
request.SetName(nil)

response, err := client.Devices.Report(ctx, request, ...)
```

## Contributing

While we value open-source contributions to this SDK, this library is generated programmatically.
Additions made directly to this library would have to be moved over to our generation code,
otherwise they would be overwritten upon the next generated release. Feel free to open a PR as
a proof of concept, but know that we will not be able to merge it as-is. We suggest opening
an issue first to discuss with us!

On the other hand, contributions to the README are always very welcome!
