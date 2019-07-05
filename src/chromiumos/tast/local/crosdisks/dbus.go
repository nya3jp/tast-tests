// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides an interface to talk to cros_disks service
// via D-Bus.
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

// CrosDisks is used to interact with the CrosDisks process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/cros-disks/dbus_bindings/org.chromium.CrosDisks.xml.
type CrosDisks struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// New connects to CrosDisks via D-Bus and returns a crosDisks object.
func New(ctx context.Context) (*CrosDisks, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &CrosDisks{conn, obj}, nil
}

// call is a thin wrapper of CallWithContext for convenience.
func (c *CrosDisks) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// EnumerateDevices calls CrosDisks.EnumerateDevices D-Bus method.
func (c *CrosDisks) EnumerateDevices(ctx context.Context) ([]string, error) {
	var ds []string
	if err := c.call(ctx, "EnumerateDevices").Store(&ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// GetDeviceProperties calls CrosDisks.GetDeviceProperties D-Bus method.
func (c *CrosDisks) GetDeviceProperties(ctx context.Context, devicePath string) (map[string]dbus.Variant, error) {
	var prop map[string]dbus.Variant
	if err := c.call(ctx, "GetDeviceProperties", devicePath).Store(&prop); err != nil {
		return nil, err
	}
	return prop, nil
}

// Mount calls CrosDisks.Mount D-Bus method.
func (c *CrosDisks) Mount(ctx context.Context, devicePath, fsType string, options []string) error {
	return c.call(ctx, "Mount", devicePath, fsType, options).Err
}

// Unmount calls CrosDisks.Unmount D-Bus method.
func (c *CrosDisks) Unmount(ctx context.Context, devicePath string, options []string) (uint32, error) {
	var status uint32
	if err := c.call(ctx, "Unmount", devicePath, options).Store(&status); err != nil {
		return 0, err
	}
	return status, nil
}

// MountCompletedWatcher is a thin wrapper of dbusutil.SignalWatcher to return
// signal content as mountCompleted.
type MountCompletedWatcher struct {
	dbusutil.SignalWatcher
}

// See MountErrorType defined in system_api/dbus/cros-disks/dbus-constants.h
const (
	MountErrorNone              uint32 = 0
	MountErrorPathNotMounted    uint32 = 6
	MountErrorInvalidDevicePath uint32 = 100
)

// MountCompleted holds the body data of MountCompleted signal.
type MountCompleted struct {
	Status     uint32
	SourcePath string
	SourceType uint32
	MountPath  string
}

// Wait waits for the MountCompleted signal, and returns the body data of the
// received signal.
func (m *MountCompletedWatcher) Wait(ctx context.Context) (MountCompleted, error) {
	var ret MountCompleted
	select {
	case s := <-m.Signals:
		if err := dbus.Store(s.Body, &ret.Status, &ret.SourcePath, &ret.SourceType, &ret.MountPath); err != nil {
			return ret, errors.Wrap(err, "failed to store MountCompleted data")
		}
		return ret, nil
	case <-ctx.Done():
		return ret, errors.Wrap(ctx.Err(), "didn't get MountCompleted signal")
	}
}

// WatchMountCompleted registers the signal watching and returns its watcher
// instance.
func (c *CrosDisks) WatchMountCompleted(ctx context.Context) (*MountCompletedWatcher, error) {
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
	return &MountCompletedWatcher{*w}, nil
}
