//go:build js && wasm

package api

import (
	"context"

	"github.com/joeblew999/charter/go/d1"
)

// D1Store is the two tables (migrations/) on the D1 binding itself: one awaited call per statement,
// and the rows reach Go as one string (package d1). It is what runs on Cloudflare.
type D1Store struct{ DB d1.DB }

const deviceColumns = "id, ts, received, due, reason, next_s, name, os, report"

func (s D1Store) Put(_ context.Context, row DeviceRow, forget int64) (bool, error) {
	kept, err := d1.Query[DeviceRow](s.DB, "INSERT INTO device_reports (id, ts, received, report) VALUES (?, ?, ?, ?) ON CONFLICT (id, ts) DO NOTHING RETURNING id", row.ID, row.TS, row.Received, row.Report)
	if err != nil || len(kept) == 0 {
		return false, err
	}
	// The device's row follows its newest report by the device's clock: a resent older one is
	// history only.
	if _, err := d1.Query[DeviceRow](s.DB, "INSERT INTO devices ("+deviceColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) "+
		"ON CONFLICT (id) DO UPDATE SET ts = excluded.ts, received = excluded.received, due = excluded.due, reason = excluded.reason, "+
		"next_s = excluded.next_s, name = excluded.name, os = excluded.os, report = excluded.report WHERE excluded.ts >= devices.ts",
		row.ID, row.TS, row.Received, row.Due, row.Reason, row.NextS, row.Name, row.OS, row.Report); err != nil {
		return true, err
	}
	_, err = d1.Query[DeviceRow](s.DB, "DELETE FROM device_reports WHERE received < ?", forget)
	return true, err
}

func (s D1Store) Device(_ context.Context, id string) (DeviceRow, bool, error) {
	rows, err := d1.Query[DeviceRow](s.DB, "SELECT "+deviceColumns+" FROM devices WHERE id = ?", id)
	if err != nil || len(rows) == 0 {
		return DeviceRow{}, false, err
	}
	return rows[0], true, nil
}

func (s D1Store) Devices(context.Context) ([]DeviceRow, error) {
	return d1.Query[DeviceRow](s.DB, "SELECT "+deviceColumns+" FROM devices ORDER BY id")
}

func (s D1Store) Reports(_ context.Context, id string, since int64, limit int) ([]DeviceRow, error) {
	return d1.Query[DeviceRow](s.DB, "SELECT id, ts, received, report FROM device_reports WHERE id = ? AND received >= ? ORDER BY ts DESC LIMIT ?", id, since, limit)
}

func (s D1Store) Forget(_ context.Context, id string) (int, bool, error) {
	reports, err := d1.Query[DeviceRow](s.DB, "DELETE FROM device_reports WHERE id = ? RETURNING id", id)
	if err != nil {
		return 0, false, err
	}
	devices, err := d1.Query[DeviceRow](s.DB, "DELETE FROM devices WHERE id = ? RETURNING id", id)
	return len(reports), len(devices) > 0, err
}

func (s D1Store) Machine(_ context.Context, token string) (string, bool, error) {
	rows, err := d1.Query[struct {
		Device string `json:"device"`
	}](s.DB, "SELECT device FROM machines WHERE token = ?", token)
	if err != nil || len(rows) == 0 {
		return "", false, err
	}
	return rows[0].Device, true, nil
}
