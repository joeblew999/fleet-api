# Reference
## Devices
<details><summary><code>client.Devices.List() -> *fleet.DeviceList</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
client.Devices.List(
    context.TODO(),
)
```
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.Devices.Get(ID) -> *fleet.DeviceView</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &fleet.GetDevicesRequest{
    ID: "3f9a1c0b7d2e4a65",
}
client.Devices.Get(
    context.TODO(),
    request,
)
```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**id:** `string` — The machine id: 16 lower-case hex digits
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.Devices.History(ID) -> *fleet.DeviceHistory</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
request := &fleet.HistoryDevicesRequest{
    ID: "3f9a1c0b7d2e4a65",
    Since: fleet.Int64(
        int64(0),
    ),
    Limit: fleet.Int(
        50,
    ),
}
client.Devices.History(
    context.TODO(),
    request,
)
```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**id:** `string` — The machine id: 16 lower-case hex digits
    
</dd>
</dl>

<dl>
<dd>

**since:** `*int64` — Only reports received at or after this, Unix milliseconds; 0: all that are kept
    
</dd>
</dl>

<dl>
<dd>

**limit:** `*int` — At most this many, newest first
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.Devices.Report(ID, request) -> *fleet.DevicePosted</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Stored as posted. The same id and ts again is a duplicate and changes nothing, so a machine can resend what it could not deliver.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
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
    },
}
client.Devices.Report(
    context.TODO(),
    request,
)
```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**id:** `string` — The machine id; must be the report's id
    
</dd>
</dl>

<dl>
<dd>

**request:** `*fleet.DeviceReport` 
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Meta
<details><summary><code>client.Meta.Hello() -> *fleet.HelloOutputBody</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```go
client.Meta.Hello(
    context.TODO(),
)
```
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

