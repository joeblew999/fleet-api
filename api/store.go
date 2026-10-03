package api

import (
	"context"
	"sort"
	"sync"
)

// Store is where reports are kept: D1 on Cloudflare (D1Store, store_js.go, the tables in
// migrations/), memory for `go run .` and the tests (MemStore).
type Store interface {
	// Put keeps a report and, when it is the device's newest, makes it the device's row; then it
	// forgets reports received before forget. False when this id and ts were already kept.
	Put(ctx context.Context, row DeviceRow, forget int64) (bool, error)
	// Device is a device's row: its newest report.
	Device(ctx context.Context, id string) (DeviceRow, bool, error)
	// Devices is every device's row, by id.
	Devices(ctx context.Context) ([]DeviceRow, error)
	// Reports is a device's reports received at or after since, newest first, at most limit.
	Reports(ctx context.Context, id string, since int64, limit int) ([]DeviceRow, error)
	// Forget removes a device's row and its reports: how many reports went, and whether it had a row.
	Forget(ctx context.Context, id string) (int, bool, error)
	// Machine is the device a machine's Access service token (its Client ID) was enrolled for: the
	// machines table, written by mise run access:token -- create.
	Machine(ctx context.Context, token string) (device string, enrolled bool, err error)
}

// DeviceRow is a report as kept: as posted (Report), with the columns worked out from it. The json
// names are the columns'.
type DeviceRow struct {
	ID       string `json:"id"`
	TS       int64  `json:"ts"`
	Received int64  `json:"received"`
	Due      int64  `json:"due"`
	Reason   string `json:"reason"`
	NextS    int64  `json:"next_s"`
	Name     string `json:"name"`
	OS       string `json:"os"`
	Report   string `json:"report"`
}

// MemStore is a Store in one process: for `go run .` and the tests.
type MemStore struct {
	mu       sync.Mutex
	devices  map[string]DeviceRow
	reports  []DeviceRow
	machines map[string]string
}

// Enrol ties a service token's Client ID to a device, as access:token create does in D1.
func (m *MemStore) Enrol(token, device string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.machines == nil {
		m.machines = map[string]string{}
	}
	m.machines[token] = device
}

func (m *MemStore) Machine(_ context.Context, token string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, ok := m.machines[token]
	return device, ok, nil
}

func (m *MemStore) Put(_ context.Context, row DeviceRow, forget int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reports {
		if r.ID == row.ID && r.TS == row.TS {
			return false, nil
		}
	}
	m.reports = append(m.reports, row)
	if m.devices == nil {
		m.devices = map[string]DeviceRow{}
	}
	if current, ok := m.devices[row.ID]; !ok || row.TS >= current.TS {
		m.devices[row.ID] = row
	}
	kept := m.reports[:0]
	for _, r := range m.reports {
		if r.Received >= forget {
			kept = append(kept, r)
		}
	}
	m.reports = kept
	return true, nil
}

func (m *MemStore) Device(_ context.Context, id string) (DeviceRow, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.devices[id]
	return row, ok, nil
}

func (m *MemStore) Devices(context.Context) ([]DeviceRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []DeviceRow{}
	for _, row := range m.devices {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *MemStore) Reports(_ context.Context, id string, since int64, limit int) ([]DeviceRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []DeviceRow{}
	for _, r := range m.reports {
		if r.ID == id && r.Received >= since {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS > out[j].TS })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemStore) Forget(_ context.Context, id string) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, found := m.devices[id]
	delete(m.devices, id)
	kept, gone := m.reports[:0], 0
	for _, r := range m.reports {
		if r.ID == id {
			gone++
			continue
		}
		kept = append(kept, r)
	}
	m.reports = kept
	return gone, found, nil
}
