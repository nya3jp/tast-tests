// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosdisks

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.CrosDisks"
	dbusPath      = "/org/chromium/CrosDisks"
	dbusInterface = "org.chromium.CrosDisks"
)

// crosDisks is used to interact with the CrosDisks process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/cros-disks/dbus_bindings/org.chromium.CrosDisks.xml.
type crosDisks struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// newCrosDisks connects to CrosDisks via D-Bus and returns a crosDisks object.
func newCrosDisks(ctx context.Context) (*crosDisks, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &crosDisks{conn, obj}, nil
}

// call is thin wrapper of CallWithContext for convenience.
func (c *crosDisks) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// enumerateDevices calls CrosDisks.EnumerateDevices D-Bus method.
func (c *crosDisks) enumerateDevices(ctx context.Context) ([]string, error) {
	var ds []string
	if err := c.call(ctx, "EnumerateDevices").Store(&ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// getDeviceProperties calls CrosDisks.GetDeviceProperties D-Bus method.
func (c *crosDisks) getDeviceProperties(ctx context.Context, devicePath string) (map[string]dbus.Variant, error) {
	var prop map[string]dbus.Variant
	if err := c.call(ctx, "GetDeviceProperties", devicePath).Store(&prop); err != nil {
		return nil, err
	}
	return prop, nil
}

// mount calls CrosDisks.Mount D-Bus method.
func (c *crosDisks) mount(ctx context.Context, devicePath, fsType string, options []string) error {
	return c.call(ctx, "Mount", devicePath, fsType, options).Err
}

// unmount calls CrosDisks.Unmount D-Bus method.
func (c *crosDisks) unmount(ctx context.Context, devicePath string, options []string) (uint32, error) {
	var status uint32
	if err := c.call(ctx, "Unmount", devicePath, options).Store(&status); err != nil {
		return 0, err
	}
	return status, nil
}

// mountCompletedWatcher is a thin wrapper of dbusutil.SignalWatcher to return
// signal content as mountCompleted.
type mountCompletedWatcher struct {
	dbusutil.SignalWatcher
}

// mountCompleted holds the body data of MountCompleted signal.
type mountCompleted struct {
	status     uint32
	sourcePath string
	sourceType uint32
	mountPath  string
}

// wait waits for the MountCompleted signal, and returns the body data of the
// received signal.
func (m *mountCompletedWatcher) wait(ctx context.Context) (mountCompleted, error) {
	var ret mountCompleted
	select {
	case s := <-m.Signals:
		if err := dbus.Store(s.Body, &ret.status, &ret.sourcePath, &ret.sourceType, &ret.mountPath); err != nil {
			return ret, errors.Wrapf(err, "failed to store MountCompleted data")
		}
		return ret, nil
	case <-ctx.Done():
		return ret, errors.Wrapf(ctx.Err(), "didn't get MountCompleted signal")
	}
}

// watchMountCompleted registeres the signal watching and returns its watcher
// instance.
func (c *crosDisks) watchMountCompleted(ctx context.Context) (*mountCompletedWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "MountCompleted",
	}
	w, err := dbusutil.NewSignalWatcher(ctx, c.conn, spec)
	if err != nil {
		return nil, err
	}
	return &mountCompletedWatcher{*w}, nil
}
