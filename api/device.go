package api

// Devices: what each machine in the fleet says about itself, and what the Worker works out from it.
//
// The tags are Huma's: they are the schema, and Huma enforces them before a handler runs. Validate
// is the rules a tag cannot say: a section that is ok has its values, one that is not has none, one
// that is unknown says why, and the values agree with each other.

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// DeviceSchema is the report's schema number. A new optional field or section does not change it;
// a field whose meaning changes, or one removed, does.
const DeviceSchema = 1

// Limits the Worker enforces on a posted report (the tags say the same to a client).
const (
	DeviceMaxBody   = 16 << 10
	DeviceMaxNextS  = 86400
	DeviceKeepDays  = 7
	DeviceKeepMilli = DeviceKeepDays * 24 * 60 * 60 * 1000
)

// Every section answers one of three things, never a zero that looks like data: ok (the values are
// there), none (this machine has no such thing: a desktop has no battery or lid) or unknown (it
// could not be read; Why says why).
const (
	DeviceOK      = "ok"
	DeviceNone    = "none"
	DeviceUnknown = "unknown"
)

// Why a report was sent.
const (
	DeviceReasonStart    = "start"    // the watcher started
	DeviceReasonChange   = "change"   // something in it changed
	DeviceReasonInterval = "interval" // nothing changed for NextS
	DeviceReasonStop     = "stop"     // the watcher is stopping on purpose
	DeviceReasonOnce     = "once"     // a single read, nothing is watching
)

// DeviceReport is one machine's state at one moment.
type DeviceReport struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Schema int32  `json:"schema" minimum:"1" doc:"The report's schema number; 1 today. The Worker refuses one it does not know."`
	ID     string `json:"id" pattern:"^[0-9a-f]{16}$" doc:"The machine id: 16 random hex digits made once per machine. Derived from nothing about the machine."`
	TS     int64  `json:"ts" minimum:"1" doc:"When the device read this, by its own clock, Unix milliseconds. Lateness is judged by when the Worker received it, never by this."`
	Reason string `json:"reason" enum:"start,change,interval,stop,once" doc:"Why it was sent."`
	NextS  int64  `json:"next_s" minimum:"0" maximum:"86400" doc:"The device promises its next report within this many seconds; 0 promises none (reason stop or once)."`

	Tool    DeviceTool    `json:"tool"`
	Host    DeviceHost    `json:"host"`
	CPU     DeviceCPU     `json:"cpu"`
	Memory  DeviceMemory  `json:"memory"`
	Disks   []DeviceDisk  `json:"disks" minItems:"1" maxItems:"8" doc:"The system volume and the volume the tool keeps its data on; one entry with both roles when they are the same volume."`
	Power   DevicePower   `json:"power"`
	Battery DeviceBattery `json:"battery"`
	Lid     DeviceLid     `json:"lid"`
	Sleep   DeviceSleep   `json:"sleep"`
	Keeper  DeviceKeeper  `json:"keeper"`
	Rig     *DeviceRig    `json:"rig,omitempty" doc:"The Claude worker claude-rig set up; absent when the reporting tool does not know."`
	Claims  *DeviceClaims `json:"claims,omitempty" doc:"Who holds the machine now; absent when the reporting tool does not know."`
	VMs     *DeviceVMs    `json:"vms,omitempty" doc:"The virtual machines on this machine; absent when the reporting tool does not know."`
}

// DeviceTool is the program that sent the report.
type DeviceTool struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Name    string `json:"name,omitempty" maxLength:"200" doc:"The program: claude-rig, irgo-winvm."`
	Version string `json:"version" minLength:"1" maxLength:"200" doc:"Its version, or the commit it runs."`
	Command string `json:"command" minLength:"1" maxLength:"200" doc:"The command that read it: doctor, device, device-watch, awake."`
}

// DeviceHost is what the machine is. Name, OS and Arch are always known.
type DeviceHost struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Name      string `json:"name" minLength:"1" maxLength:"200" doc:"The hostname's first label."`
	OS        string `json:"os" enum:"darwin,linux,windows" doc:"GOOS."`
	Arch      string `json:"arch" minLength:"1" maxLength:"200" doc:"GOARCH: arm64, amd64."`
	OSName    string `json:"os_name,omitempty" maxLength:"200" doc:"macOS, Ubuntu, Windows 11 Pro."`
	OSVersion string `json:"os_version,omitempty" maxLength:"200"`
	Model     string `json:"model,omitempty" maxLength:"200" doc:"The hardware model, Mac14,10. Never a serial number."`
	Guest     *bool  `json:"guest,omitempty" doc:"True in a virtual machine; absent when that cannot be told."`
	Boot      int64  `json:"boot,omitempty" minimum:"0" doc:"When it booted, Unix milliseconds."`
}

// DeviceCPU is the processors and how busy they are.
type DeviceCPU struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status string   `json:"status" enum:"ok,unknown"`
	Why    string   `json:"why,omitempty" maxLength:"200"`
	Count  *int32   `json:"count,omitempty" minimum:"1" doc:"Logical processors."`
	Load1  *float64 `json:"load1,omitempty" minimum:"0" doc:"The one-minute load average; absent on Windows, which has none."`
}

// DeviceMemory is physical memory, in bytes.
type DeviceMemory struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status    string `json:"status" enum:"ok,unknown"`
	Why       string `json:"why,omitempty" maxLength:"200"`
	Total     *int64 `json:"total,omitempty" minimum:"1"`
	Available *int64 `json:"available,omitempty" minimum:"0" doc:"What a new program could have without swapping."`
}

// DeviceDisk is one volume, in bytes.
type DeviceDisk struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Roles  []string `json:"roles" minItems:"1" maxItems:"2" enum:"system,data" doc:"system: the OS's volume. data: where the tool keeps its work."`
	Path   string   `json:"path" minLength:"1" maxLength:"200" doc:"Where it is mounted, the home directory written as ~."`
	Status string   `json:"status" enum:"ok,unknown"`
	Why    string   `json:"why,omitempty" maxLength:"200"`
	FS     string   `json:"fs,omitempty" maxLength:"200" doc:"apfs, ext4, NTFS."`
	Total  *int64   `json:"total,omitempty" minimum:"1"`
	Free   *int64   `json:"free,omitempty" minimum:"0" doc:"What this user can still write."`
}

// DevicePower is what the machine is running on.
type DevicePower struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status string `json:"status" enum:"ok,unknown"`
	Why    string `json:"why,omitempty" maxLength:"200"`
	Source string `json:"source,omitempty" enum:"ac,battery,ups"`
}

// DeviceBattery is every battery in the machine taken as one.
type DeviceBattery struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status     string   `json:"status" enum:"ok,none,unknown"`
	Why        string   `json:"why,omitempty" maxLength:"200"`
	Count      *int32   `json:"count,omitempty" minimum:"1"`
	Percent    *float64 `json:"percent,omitempty" minimum:"0" maximum:"100" doc:"Charge, as the OS shows it."`
	State      string   `json:"state,omitempty" enum:"charging,discharging,full,idle" doc:"idle: on external power, neither charging nor full (a charge limit)."`
	Health     *float64 `json:"health,omitempty" minimum:"0" maximum:"150" doc:"Full capacity as a percent of design capacity; absent when the OS does not say."`
	RemainingS *int64   `json:"remaining_s,omitempty" minimum:"0" doc:"The OS's estimate to empty or to full, seconds; absent when it has none."`
}

// DeviceLid is the laptop's lid.
type DeviceLid struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status string `json:"status" enum:"ok,none,unknown"`
	Why    string `json:"why,omitempty" maxLength:"200"`
	Closed *bool  `json:"closed,omitempty"`
}

// DeviceSleep is when the machine would go to sleep by itself.
type DeviceSleep struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status     string   `json:"status" enum:"ok,none,unknown" doc:"none: a machine that cannot sleep (a container, a CI runner)."`
	Why        string   `json:"why,omitempty" maxLength:"200"`
	IdleS      *int64   `json:"idle_s,omitempty" minimum:"0" doc:"Idle seconds before it sleeps on the current power source; 0 is never."`
	DisplayS   *int64   `json:"display_s,omitempty" minimum:"0" doc:"Idle seconds before the display turns off; 0 is never."`
	LidAction  string   `json:"lid_action,omitempty" enum:"sleep,hibernate,shutdown,lock,nothing" doc:"What closing the lid does now. nothing is a machine that keeps running closed."`
	Inhibited  *bool    `json:"inhibited,omitempty" doc:"Something is holding idle sleep off right now."`
	Inhibitors []string `json:"inhibitors,omitempty" maxItems:"8" maxLength:"200" doc:"The programs holding it off, by process name only."`
}

// DeviceKeeper is the reporting tool's own keep-awake (irgo-winvm awake).
type DeviceKeeper struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status  string `json:"status" enum:"ok,none,unknown" doc:"none: the tool has no keep-awake for this OS."`
	Why     string `json:"why,omitempty" maxLength:"200"`
	Running *bool  `json:"running,omitempty"`
	Idle    *bool  `json:"idle,omitempty" doc:"It is holding idle sleep off."`
	Lid     *bool  `json:"lid,omitempty" doc:"It has set closing the lid to do nothing."`
}

// DeviceRig is the machine as a Claude worker: what claude-rig's doctor reads.
type DeviceRig struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status         string `json:"status" enum:"ok,none,unknown" doc:"none: claude-rig never set this machine up."`
	Why            string `json:"why,omitempty" maxLength:"200"`
	Commit         string `json:"commit,omitempty" maxLength:"64" doc:"The claude-rig commit it last ran; absent when not known."`
	ToolsInstalled *bool  `json:"tools_installed,omitempty" doc:"Every tool the rig installs is there."`
	ClaudeVersion  string `json:"claude_version,omitempty" maxLength:"200" doc:"What claude --version prints; absent when Claude is not installed."`
	ConfigApplied  *bool  `json:"config_applied,omitempty" doc:"The rig's Claude configuration is applied."`
	LoggedIn       *bool  `json:"logged_in,omitempty" doc:"Claude is logged in."`
	SessionRunning *bool  `json:"session_running,omitempty" doc:"A Claude session is running for the rig."`
	WorkDir        string `json:"work_dir,omitempty" maxLength:"200" doc:"Where its jobs run, the home directory written as ~."`
}

// DeviceClaims is who holds the machine: the claims taken on it, which are the authority, as the
// machine last saw them.
type DeviceClaims struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status string        `json:"status" enum:"ok,none,unknown" doc:"none: this machine takes no claims."`
	Why    string        `json:"why,omitempty" maxLength:"200"`
	Slots  *int32        `json:"slots,omitempty" minimum:"1" maximum:"64" doc:"How many jobs it takes at once."`
	Held   []DeviceClaim `json:"held,omitempty" maxItems:"64" doc:"The claims held now; empty or absent when the machine is free."`
}

// DeviceClaim is one job holding a slot on the machine.
type DeviceClaim struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	ID     string `json:"id" pattern:"^[A-Za-z0-9._-]{1,64}$" doc:"The claim's id on the machine."`
	Caller string `json:"caller" minLength:"1" maxLength:"200" doc:"Who holds it: an agent, a session or a tool. Never a person's user name or address, so no @."`
	Job    string `json:"job" minLength:"1" maxLength:"200" doc:"What it is doing."`
	Since  int64  `json:"since" minimum:"1" doc:"When it was taken, Unix milliseconds, by the machine's clock."`
	Until  int64  `json:"until,omitempty" minimum:"0" doc:"When it lapses unless renewed, Unix milliseconds; absent when it does not lapse."`
}

// DeviceVMs is the virtual machines on the machine, as the tool that manages them sees them.
type DeviceVMs struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Status         string     `json:"status" enum:"ok,none,unknown" doc:"none: no VM manager on this machine."`
	Why            string     `json:"why,omitempty" maxLength:"200"`
	Manager        string     `json:"manager,omitempty" maxLength:"200" doc:"What runs them: utm."`
	ManagerRunning *bool      `json:"manager_running,omitempty" doc:"The manager is running. When it is not, every VM is stopped, and the list is what the reporting tool knows of them."`
	List           []DeviceVM `json:"list,omitempty" maxItems:"32"`
}

// DeviceVM is one virtual machine.
type DeviceVM struct {
	_ struct{} `json:"-" additionalProperties:"true"`

	Name         string `json:"name" minLength:"1" maxLength:"200"`
	State        string `json:"state" minLength:"1" maxLength:"200" doc:"As the manager says it: started, stopped, paused, starting, stopping."`
	OS           string `json:"os,omitempty" enum:"windows,linux,darwin" doc:"The system inside; absent when not known."`
	Owner        string `json:"owner,omitempty" maxLength:"200" doc:"Who made it: an agent, a repository or a tool. Never a person's user name or address, so no @."`
	KeepRunning  bool   `json:"keep_running,omitempty" doc:"The machine starts it again whenever it stops."`
	KeeperStarts int32  `json:"keeper_starts,omitempty" minimum:"0" doc:"How many times the machine's keeper has started it since the keeper began."`
}

// Conditions the Worker works out. Notifications will announce these.
const (
	DeviceCondStopped         = "stopped"           // the newest report's reason is stop
	DeviceCondQuiet           = "quiet"             // a promised report is more than three NextS late
	DeviceCondUnminded        = "unminded"          // lid_action is nothing and the keeper is not running, or the device is quiet or stopped
	DeviceCondClosedOnBattery = "closed-on-battery" // lid closed, on battery, and still reporting: awake in a bag
	DeviceCondBatteryLow      = "battery-low"       // on battery under 20 percent
	DeviceCondRigUnready      = "rig-unready"       // claude-rig set it up, and something it needs is missing
)

// DeviceCondition is one thing about a device someone should know.
type DeviceCondition struct {
	Code  string `json:"code" enum:"stopped,quiet,unminded,closed-on-battery,battery-low,rig-unready"`
	Since int64  `json:"since" doc:"When it began to hold, as far as the Worker knows: when the report that shows it was received, or for quiet when the report became late. Unix milliseconds."`
	Why   string `json:"why" doc:"One sentence, with the numbers."`
}

// PostedReport is a report exactly as it was posted: a newer tool's fields survive, which decoding
// into DeviceReport and encoding again would drop. Its schema is DeviceReport's.
type PostedReport json.RawMessage

func (p PostedReport) MarshalJSON() ([]byte, error) { return p, nil }
func (p *PostedReport) UnmarshalJSON(b []byte) error {
	*p = append((*p)[:0], b...)
	return nil
}
func (PostedReport) Schema(r huma.Registry) *huma.Schema {
	return r.Schema(reflect.TypeFor[DeviceReport](), true, "")
}

// DeviceView is a device as the Worker last heard from it.
type DeviceView struct {
	Report     PostedReport      `json:"report" doc:"The newest report, as posted."`
	Received   int64             `json:"received" doc:"When the Worker received it, Unix milliseconds."`
	Due        int64             `json:"due,omitempty" doc:"When the next was promised by: received + next_s. Absent when none was."`
	Conditions []DeviceCondition `json:"conditions" doc:"Empty when there is nothing to say."`
}

// DeviceList is every device.
type DeviceList struct {
	Now     int64        `json:"now" doc:"The Worker's clock, Unix milliseconds: what due and quiet are judged against."`
	Devices []DeviceView `json:"devices" doc:"By id."`
}

// DevicePosted is the answer to a posted report.
type DevicePosted struct {
	ID         string            `json:"id"`
	Received   int64             `json:"received"`
	Duplicate  bool              `json:"duplicate" doc:"This id and ts were already stored; nothing changed."`
	Conditions []DeviceCondition `json:"conditions"`
}

// DeviceHistory is a device's reports, newest first.
type DeviceHistory struct {
	ID      string         `json:"id"`
	Reports []DeviceStored `json:"reports"`
}

// DeviceStored is a report as posted, with when it arrived.
type DeviceStored struct {
	Report   PostedReport `json:"report"`
	Received int64        `json:"received"`
}

// FieldError is one thing wrong with a report: where, and what.
type FieldError struct {
	Path string
	Msg  string
}

// Validate is the rules a report must meet that its tags cannot say.
func (r *DeviceReport) Validate() []FieldError {
	var e []FieldError
	bad := func(path, msg string) { e = append(e, FieldError{path, msg}) }
	// section checks why against status, and says whether values are expected.
	section := func(path, status, why string) bool {
		if status == DeviceUnknown && why == "" {
			bad(path+".why", "unknown needs a reason")
		}
		if status == DeviceOK && why != "" {
			bad(path+".why", "ok has no reason")
		}
		return status == DeviceOK
	}
	need := func(ok bool, path string, present bool) {
		switch {
		case ok && !present:
			bad(path, "missing though the section is ok")
		case !ok && present:
			bad(path, "present though the section is not ok")
		}
	}
	only := func(ok bool, path string, present bool) { // optional when ok, forbidden otherwise
		if !ok && present {
			bad(path, "present though the section is not ok")
		}
	}
	notHome := func(path, value string) {
		if homeDir(value) {
			bad(path, "names a home directory: write it as ~")
		}
	}

	if r.Schema != DeviceSchema {
		bad("schema", "not a schema this version knows")
	}
	watching := r.Reason == DeviceReasonStart || r.Reason == DeviceReasonChange || r.Reason == DeviceReasonInterval
	switch {
	case watching && r.NextS == 0:
		bad("next_s", "a watcher promises its next report")
	case !watching && r.NextS != 0:
		bad("next_s", "stop and once promise nothing")
	}

	ok := section("cpu", r.CPU.Status, r.CPU.Why)
	need(ok, "cpu.count", r.CPU.Count != nil)
	only(ok, "cpu.load1", r.CPU.Load1 != nil)

	ok = section("memory", r.Memory.Status, r.Memory.Why)
	need(ok, "memory.total", r.Memory.Total != nil)
	need(ok, "memory.available", r.Memory.Available != nil)
	if ok && r.Memory.Total != nil && r.Memory.Available != nil && *r.Memory.Available > *r.Memory.Total {
		bad("memory.available", "more than total")
	}

	seen := map[string]bool{}
	for i, d := range r.Disks {
		p := "disks[" + itoa(i) + "]"
		for _, role := range d.Roles {
			if seen[role] {
				bad(p+".roles", "a role is given twice")
			}
			seen[role] = true
		}
		notHome(p+".path", d.Path)
		ok := section(p, d.Status, d.Why)
		need(ok, p+".total", d.Total != nil)
		need(ok, p+".free", d.Free != nil)
		if ok && d.Total != nil && d.Free != nil && *d.Free > *d.Total {
			bad(p+".free", "more than total")
		}
	}
	if !seen["system"] || !seen["data"] {
		bad("disks", "the system volume and the data volume are both required, readable or not")
	}

	ok = section("power", r.Power.Status, r.Power.Why)
	need(ok, "power.source", r.Power.Source != "")

	b := r.Battery
	ok = section("battery", b.Status, b.Why)
	need(ok, "battery.count", b.Count != nil)
	need(ok, "battery.percent", b.Percent != nil)
	need(ok, "battery.state", b.State != "")
	only(ok, "battery.health", b.Health != nil)
	only(ok, "battery.remaining_s", b.RemainingS != nil)
	if b.Percent != nil && !(*b.Percent >= 0 && *b.Percent <= 100) { // NaN as well
		bad("battery.percent", "outside 0 to 100")
	}
	if ok && r.Power.Source == "battery" && b.State == "charging" {
		bad("battery.state", "charging while the power source is the battery")
	}
	if b.Status == DeviceNone && r.Power.Source == "battery" {
		bad("power.source", "battery, with no battery")
	}

	ok = section("lid", r.Lid.Status, r.Lid.Why)
	need(ok, "lid.closed", r.Lid.Closed != nil)

	s := r.Sleep
	ok = section("sleep", s.Status, s.Why)
	need(ok, "sleep.idle_s", s.IdleS != nil)
	need(ok, "sleep.inhibited", s.Inhibited != nil)
	only(ok, "sleep.display_s", s.DisplayS != nil)
	only(ok, "sleep.lid_action", s.LidAction != "")
	only(ok, "sleep.inhibitors", len(s.Inhibitors) > 0)
	if ok && r.Lid.Status == DeviceOK && s.LidAction == "" {
		bad("sleep.lid_action", "missing though there is a lid")
	}
	if s.Inhibited != nil && !*s.Inhibited && len(s.Inhibitors) > 0 {
		bad("sleep.inhibitors", "listed though nothing inhibits")
	}

	k := r.Keeper
	ok = section("keeper", k.Status, k.Why)
	need(ok, "keeper.running", k.Running != nil)
	need(ok, "keeper.idle", k.Idle != nil)
	need(ok, "keeper.lid", k.Lid != nil)
	if ok && k.Running != nil && !*k.Running && ((k.Idle != nil && *k.Idle) || (k.Lid != nil && *k.Lid)) {
		// A lid setting that outlives the keeper is the unminded condition, and shows as
		// sleep.lid_action nothing, not here.
		bad("keeper", "holding something though it is not running")
	}

	if g := r.Rig; g != nil {
		ok = section("rig", g.Status, g.Why)
		need(ok, "rig.tools_installed", g.ToolsInstalled != nil)
		need(ok, "rig.config_applied", g.ConfigApplied != nil)
		need(ok, "rig.logged_in", g.LoggedIn != nil)
		need(ok, "rig.session_running", g.SessionRunning != nil)
		only(ok, "rig.commit", g.Commit != "")
		only(ok, "rig.claude_version", g.ClaudeVersion != "")
		only(ok, "rig.work_dir", g.WorkDir != "")
		notHome("rig.work_dir", g.WorkDir)
	}

	if c := r.Claims; c != nil {
		ok = section("claims", c.Status, c.Why)
		need(ok, "claims.slots", c.Slots != nil)
		only(ok, "claims.held", len(c.Held) > 0)
		if ok && c.Slots != nil && int32(len(c.Held)) > *c.Slots {
			bad("claims.held", "more claims than slots")
		}
		ids := map[string]bool{}
		for i, claim := range c.Held {
			p := "claims.held[" + itoa(i) + "]"
			if ids[claim.ID] {
				bad(p+".id", "given twice")
			}
			ids[claim.ID] = true
			if strings.Contains(claim.Caller, "@") {
				bad(p+".caller", "has an @: name the agent or session, not a person")
			}
			if claim.Until != 0 && claim.Until < claim.Since {
				bad(p+".until", "before since")
			}
		}
	}
	if v := r.VMs; v != nil {
		ok = section("vms", v.Status, v.Why)
		need(ok, "vms.manager_running", v.ManagerRunning != nil)
		only(ok, "vms.manager", v.Manager != "")
		only(ok, "vms.list", len(v.List) > 0)
		names := map[string]bool{}
		for i, vm := range v.List {
			p := "vms.list[" + itoa(i) + "]"
			if names[strings.ToLower(vm.Name)] {
				bad(p+".name", "given twice")
			}
			names[strings.ToLower(vm.Name)] = true
			if strings.Contains(vm.Owner, "@") {
				bad(p+".owner", "has an @: name the agent or repository, not a person")
			}
		}
	}
	return e
}

// homeDir reports whether a path names a home directory by its owner's name: /Users/<name>,
// /home/<name>, C:\Users\<name>. ~ is how a report says it.
func homeDir(path string) bool {
	p := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
	if len(p) > 1 && p[1] == ':' {
		p = p[2:]
	}
	for _, home := range []string{"/users/", "/home/"} {
		if strings.HasPrefix(p, home) && len(p) > len(home) && !strings.HasPrefix(p[len(home):], "shared") {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
