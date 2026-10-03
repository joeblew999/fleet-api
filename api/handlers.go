package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/joeblew999/charter/go/humamcp"
	"github.com/joeblew999/charter/go/humaworkers"
)

// Env is what the platform supplies: variables and secrets (bindings on Cloudflare, platform_js.go;
// the environment natively) and the store, opened per request: a binding belongs to the request's
// environment.
type Env struct {
	Var   func(name string) string
	Store func() (Store, error)
}

// The two secrets. Unset, no token matches: the API refuses everything but hello and the specs.
const (
	WriteToken = "WRITE_TOKEN" // posts reports, and reads
	ReadToken  = "READ_TOKEN"  // reads
)

// Handler serves the contract on env, plus the spec with the request's origin as its server, plus
// the read operations as MCP tools (/api/mcp: a tool call runs the same operation as the REST
// route, with the caller's Authorization).
func Handler(env Env) http.Handler {
	routes := humaworkers.New(config(), Routes(env))
	routes.UseMiddleware(env.authorize(routes))
	mcp := humamcp.Handler(routes)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var spec func(server string) ([]byte, error)
		switch r.URL.Path {
		case "/api/openapi.json":
			spec = OpenAPI
		case "/api/asyncapi.json":
			spec = AsyncAPI
		case "/api/mcp":
			mcp.ServeHTTP(w, r)
			return
		default:
			routes.ServeHTTP(w, r)
			return
		}
		body, err := spec(origin(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
}

// origin is the request's scheme and host: absolute in r.URL on Workers, from Host on net/http.
func origin(r *http.Request) string {
	if r.URL.Scheme != "" && r.URL.Host != "" {
		return r.URL.Scheme + "://" + r.URL.Host
	}
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// authorize is the contract's security, enforced: an operation needs a token unless its Security is
// empty (hello). Posting and deleting need the write token; reading takes either.
func (env Env) authorize(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		op := ctx.Operation()
		if op.Security != nil && len(op.Security) == 0 {
			next(ctx)
			return
		}
		token := bearer(ctx.Header("Authorization"))
		allowed := env.matches(token, WriteToken) || (op.Method == http.MethodGet && env.matches(token, ReadToken))
		if !allowed {
			ctx.SetHeader("WWW-Authenticate", "Bearer")
			message := "a valid token is required: Authorization: Bearer <token>"
			if op.Method != http.MethodGet {
				message = "the write token is required: Authorization: Bearer <write token>"
			}
			huma.WriteErr(api, ctx, http.StatusUnauthorized, message)
			return
		}
		next(ctx)
	}
}

// matches reports whether token is the secret's value, in constant time. An unset secret matches nothing.
func (env Env) matches(token, secret string) bool {
	want := ""
	if env.Var != nil {
		want = env.Var(secret)
	}
	return want != "" && token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1
}

func bearer(authorization string) string {
	scheme, token, ok := strings.Cut(authorization, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func (env Env) hello(context.Context, *struct{}) (*HelloOutput, error) {
	out := &HelloOutput{}
	out.Body.Message = "Hello from " + env.Var("APP_NAME")
	return out, nil
}

// Resolve is what the tags cannot say: the report's own rules (Validate) and the id in the path.
func (in *DevicePostInput) Resolve(huma.Context) []error {
	var errs []error
	if in.Body.ID != in.ID {
		errs = append(errs, &huma.ErrorDetail{Location: "body.id", Message: "not the id in the path", Value: in.Body.ID})
	}
	for _, e := range in.Body.Validate() {
		errs = append(errs, &huma.ErrorDetail{Location: "body." + e.Path, Message: e.Msg})
	}
	return errs
}

func (env Env) devicePost(ctx context.Context, in *DevicePostInput) (*DevicePostOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	row := DeviceRow{
		ID: in.Body.ID, TS: in.Body.TS, Received: now, Reason: in.Body.Reason, NextS: in.Body.NextS,
		Name: in.Body.Host.Name, OS: in.Body.Host.OS, Report: string(in.RawBody),
	}
	if in.Body.NextS > 0 {
		row.Due = now + in.Body.NextS*1000
	}
	fresh, err := store.Put(ctx, row, now-DeviceKeepMilli)
	if err != nil {
		return nil, err
	}
	return &DevicePostOutput{Body: DevicePosted{ID: in.Body.ID, Received: now, Duplicate: !fresh, Conditions: conditions(in.Body, now, now)}}, nil
}

func (env Env) deviceGet(ctx context.Context, in *DeviceGetInput) (*DeviceOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	row, found, err := store.Device(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, huma.Error404NotFound("no machine " + in.ID + " has reported")
	}
	v, err := view(row, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	return &DeviceOutput{Body: v}, nil
}

func (env Env) deviceList(ctx context.Context, _ *struct{}) (*DevicesOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	rows, err := store.Devices(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	out := &DevicesOutput{Body: DeviceList{Now: now, Devices: []DeviceView{}}}
	for _, row := range rows {
		v, err := view(row, now)
		if err != nil {
			return nil, err
		}
		out.Body.Devices = append(out.Body.Devices, v)
	}
	return out, nil
}

func (env Env) deviceReports(ctx context.Context, in *DeviceReportsInput) (*DeviceReportsOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	rows, err := store.Reports(ctx, in.ID, in.Since, int(in.Limit))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if _, found, err := store.Device(ctx, in.ID); err != nil {
			return nil, err
		} else if !found {
			return nil, huma.Error404NotFound("no machine " + in.ID + " has reported")
		}
	}
	out := &DeviceReportsOutput{Body: DeviceHistory{ID: in.ID, Reports: []DeviceStored{}}}
	for _, row := range rows {
		out.Body.Reports = append(out.Body.Reports, DeviceStored{Report: PostedReport(row.Report), Received: row.Received})
	}
	return out, nil
}

func (env Env) deviceDelete(ctx context.Context, in *DeviceDeleteInput) (*DeviceDeleteOutput, error) {
	store, err := env.Store()
	if err != nil {
		return nil, err
	}
	reports, found, err := store.Forget(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, huma.Error404NotFound("no machine " + in.ID + " has reported")
	}
	return &DeviceDeleteOutput{Body: DeviceDeleted{ID: in.ID, Reports: int32(reports)}}, nil
}

// view is a device's row as the API shows it, with its conditions at now.
func view(row DeviceRow, now int64) (DeviceView, error) {
	var report DeviceReport
	if err := json.Unmarshal([]byte(row.Report), &report); err != nil {
		return DeviceView{}, err
	}
	return DeviceView{Report: PostedReport(row.Report), Received: row.Received, Due: row.Due, Conditions: conditions(report, row.Received, now)}, nil
}

// conditions is what someone should know about a device, from its newest report, when the Worker
// received it, and the time now. Lateness is judged by received, never by the device's clock.
func conditions(r DeviceReport, received, now int64) []DeviceCondition {
	out := []DeviceCondition{}
	add := func(code string, since int64, why string) {
		out = append(out, DeviceCondition{Code: code, Since: since, Why: why})
	}
	stopped := r.Reason == DeviceReasonStop
	late := received + 3*r.NextS*1000
	quiet := r.NextS > 0 && now > late
	if stopped {
		add(DeviceCondStopped, received, "its watcher stopped on purpose")
	}
	if quiet {
		add(DeviceCondQuiet, late, "promised a report every "+strconv.FormatInt(r.NextS, 10)+" s; none for "+strconv.FormatInt((now-received)/1000, 10)+" s")
	}
	if r.Sleep.Status == DeviceOK && r.Sleep.LidAction == "nothing" {
		switch running := r.Keeper.Running != nil && *r.Keeper.Running; {
		case !running:
			add(DeviceCondUnminded, received, "closing the lid does nothing and the keeper is not running")
		case quiet || stopped:
			add(DeviceCondUnminded, received, "closing the lid does nothing and the machine no longer reports")
		}
	}
	onBattery := r.Power.Status == DeviceOK && r.Power.Source == "battery"
	if onBattery && r.Lid.Closed != nil && *r.Lid.Closed && !quiet && !stopped {
		add(DeviceCondClosedOnBattery, received, "lid closed, on battery, and still running")
	}
	if onBattery && r.Battery.Percent != nil && *r.Battery.Percent < 20 {
		add(DeviceCondBatteryLow, received, "on battery at "+strconv.FormatFloat(*r.Battery.Percent, 'f', 0, 64)+" percent")
	}
	if g := r.Rig; g != nil && g.Status == DeviceOK {
		var missing []string
		for _, check := range []struct {
			ok   *bool
			what string
		}{{g.ToolsInstalled, "tools not installed"}, {g.ConfigApplied, "configuration not applied"}, {g.LoggedIn, "Claude not logged in"}} {
			if check.ok != nil && !*check.ok {
				missing = append(missing, check.what)
			}
		}
		if len(missing) > 0 {
			add(DeviceCondRigUnready, received, strings.Join(missing, ", "))
		}
	}
	return out
}
