// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosdisks provides an interface to talk to cros_disks service
// via D-Bus.
package crosdisks

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.CrosDisks"
	dbusPath      = "/org/chromium/CrosDisks"
	dbusInterface = "org.chromium.CrosDisks"
)

// MountError matches MountErrorType defined in
// system_api/dbus/cros-disks/dbus-constants.h
type MountError uint32

// MountError matches MountErrorType defined in
// system_api/dbus/cros-disks/dbus-constants.h
const (
	MountErrorNone               MountError = 0
	MountErrorInvalidPath        MountError = 4
	MountErrorPathNotMounted     MountError = 6
	MountErrorMountProgramFailed MountError = 12
	MountErrorInvalidDevicePath  MountError = 13
	MountErrorNeedPassword       MountError = 17
	MountErrorInProgress         MountError = 18
	MountErrorCancelled          MountError = 19
)

func (e MountError) Error() string {
	switch e {
	case MountErrorNone:
		return "MountErrorNone"
	case MountErrorInvalidPath:
		return "MountErrorInvalidPath"
	case MountErrorPathNotMounted:
		return "MountErrorPathNotMounted"
	case MountErrorMountProgramFailed:
		return "MountErrorMountProgramFailed"
	case MountErrorNeedPassword:
		return "MountErrorNeedPassword"
	case MountErrorInProgress:
		return "MountErrorInProgress"
	case MountErrorCancelled:
		return "MountErrorCancelled"
	case MountErrorInvalidDevicePath:
		return "MountErrorInvalidDevicePath"
	}

	return fmt.Sprintf("MountError(%d)", uint32(e))
}

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

// Close connection to CrosDisks D-Bus service.
func (c *CrosDisks) Close() error {
	// Do not close the connection as it's a singleton shared with other tests.
	return nil
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
func (c *CrosDisks) Unmount(ctx context.Context, devicePath string, options []string) error {
	var status MountError
	if err := c.call(ctx, "Unmount", devicePath, options).Store(&status); err != nil {
		return err
	}

	if status != MountErrorNone {
		return status
	}

	return nil
}

// Rename calls CrosDisks.Rename D-Bus method.
func (c *CrosDisks) Rename(ctx context.Context, path, volumeName string) error {
	return c.call(ctx, "Rename", path, volumeName).Err
}

// Format calls CrosDisks.Format D-Bus method.
func (c *CrosDisks) Format(ctx context.Context, path, fsType string, options []string) error {
	return c.call(ctx, "Format", path, fsType, options).Err
}

// AddDeviceToAllowlist calls CrosDisks.AddDeviceToAllowlist D-Bus method.
func (c *CrosDisks) AddDeviceToAllowlist(ctx context.Context, devicePath string) error {
	return c.call(ctx, "AddDeviceToAllowlist", devicePath).Err
}

// RemoveDeviceFromAllowlist calls CrosDisks.RemoveDeviceFromAllowlist D-Bus method.
func (c *CrosDisks) RemoveDeviceFromAllowlist(ctx context.Context, devicePath string) error {
	return c.call(ctx, "RemoveDeviceFromAllowlist", devicePath).Err
}

// MountCompletedWatcher is a thin wrapper of dbusutil.SignalWatcher to return
// signal content as MountCompleted.
type MountCompletedWatcher struct {
	dbusutil.SignalWatcher
}

// DeviceOperationCompletionWatcher is a thin wrapper of dbusutil.SignalWatcher to return
// signal content as DeviceOperationCompleted.
type DeviceOperationCompletionWatcher struct {
	dbusutil.SignalWatcher
}

// MountCompleted holds the body data of MountCompleted signal.
type MountCompleted struct {
	Status     MountError
	SourcePath string
	SourceType uint32
	MountPath  string
	ReadOnly   bool
}

// DeviceOperationCompleted holds status of the operation done.
type DeviceOperationCompleted struct {
	Status uint32
	Device string
}

// Wait waits for the MountCompleted signal, and returns the body data of the
// received signal.
func (m *MountCompletedWatcher) Wait(ctx context.Context) (MountCompleted, error) {
	var ret MountCompleted
	select {
	case s := <-m.Signals:
		if err := dbus.Store(s.Body, &ret.Status, &ret.SourcePath, &ret.SourceType, &ret.MountPath, &ret.ReadOnly); err != nil {
			return ret, errors.Wrap(err, "failed to store MountCompleted data")
		}
		return ret, nil
	case <-ctx.Done():
		return ret, errors.Wrap(ctx.Err(), "didn't get MountCompleted signal")
	}
}

// Wait waits for the DeviceOperationCompleted signal, and returns the body data of the received signal.
func (m *DeviceOperationCompletionWatcher) Wait(ctx context.Context) (DeviceOperationCompleted, error) {
	var ret DeviceOperationCompleted
	select {
	case s := <-m.Signals:
		if err := dbus.Store(s.Body, &ret.Status, &ret.Device); err != nil {
			return ret, errors.Wrap(err, "failed to store DeviceOperationCompleted data")
		}
		return ret, nil
	case <-ctx.Done():
		return ret, errors.Wrap(ctx.Err(), "didn't get DeviceOperationCompleted signal")
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

// MountAndWaitForCompletion mounts and waits for the response signal.
// This is a convenience method for the odd CrosDisks' mounting API.
func (c *CrosDisks) MountAndWaitForCompletion(ctx context.Context, devicePath, fsType string, options []string) (MountCompleted, error) {
	w, err := c.WatchMountCompleted(ctx)
	if err != nil {
		return MountCompleted{}, err
	}
	defer w.Close(ctx)

	if err := c.Mount(ctx, devicePath, fsType, options); err != nil {
		return MountCompleted{}, err
	}

	m, err := w.Wait(ctx)
	if err != nil {
		return MountCompleted{}, err
	}
	return m, nil
}

// doSomethingAndWaitForCompletion performs an operation and waits for the response signal.
// This is a convenience method for the odd CrosDisks' signal-based API.
func (c *CrosDisks) doSomethingAndWaitForCompletion(ctx context.Context, f func() error, signalName string) (DeviceOperationCompleted, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    signalName,
	}
	s, err := dbusutil.NewSignalWatcher(ctx, c.conn, spec)
	if err != nil {
		return DeviceOperationCompleted{}, err
	}
	w := DeviceOperationCompletionWatcher{*s}
	defer w.Close(ctx)

	if err := f(); err != nil {
		return DeviceOperationCompleted{}, err
	}

	r, err := w.Wait(ctx)
	if err != nil {
		return DeviceOperationCompleted{}, err
	}
	return r, nil
}

// RenameAndWaitForCompletion renames volume and waits for the response signal.
func (c *CrosDisks) RenameAndWaitForCompletion(ctx context.Context, path, volumeName string) (DeviceOperationCompleted, error) {
	return c.doSomethingAndWaitForCompletion(ctx, func() error { return c.Rename(ctx, path, volumeName) }, "RenameCompleted")
}

// FormatAndWaitForCompletion formats volume and waits for the response signal.
func (c *CrosDisks) FormatAndWaitForCompletion(ctx context.Context, path, fsType string, options []string) (DeviceOperationCompleted, error) {
	return c.doSomethingAndWaitForCompletion(ctx, func() error { return c.Format(ctx, path, fsType, options) }, "FormatCompleted")
}
