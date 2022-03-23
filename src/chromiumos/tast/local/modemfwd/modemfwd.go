// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package modemfwd interacts with modemfwd D-Bus service.
package modemfwd

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

const (
	dbusPath      = "/org/chromium/Modemfwd"
	dbusName      = "org.chromium.Modemfwd"
	dbusInterface = "org.chromium.Modemfwd"
	// JobName is the name of the modemfwd process
	JobName = "modemfwd"
	// DisableAutoUpdatePref disables auto update on modemfwd
	DisableAutoUpdatePref = "/var/lib/modemfwd/disable_auto_update"
)

// Modemfwd is used to interact with the modemfwd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/modemfwd/dbus_bindings/org.chromium.Modemfwd.xml
type Modemfwd struct {
	*dbusutil.DBusObject
}

// New connects to modemfwd via D-Bus and returns a Modemfwd object.
func New(ctx context.Context) (*Modemfwd, error) {
	obj, err := dbusutil.NewDBusObject(ctx, dbusName, dbusInterface, dbusPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to modemfwd")
	}
	return &Modemfwd{obj}, nil
}

// ForceFlash calls modemfwd's ForceFlash D-Bus method.
func (m *Modemfwd) ForceFlash(ctx context.Context, device string, options map[string]string) error {
	result := false
	if err := m.Call(ctx, "ForceFlash", device, options).Store(&result); err != nil {
		return err
	}
	if !result {
		return errors.New("ForceFlash returned false")
	}
	return nil
}

// UpdateFirmwareCompletedSignal holds values created from the MemoryPressureChrome D-Bus
// signal.
type UpdateFirmwareCompletedSignal struct {
	success bool
	errStr  string
}

func parseUpdateFirmwareCompletedSignal(sig *dbus.Signal) (UpdateFirmwareCompletedSignal, error) {
	res := UpdateFirmwareCompletedSignal{}
	if len(sig.Body) != 2 {
		return res, errors.Errorf("expected 2 params, got %d", len(sig.Body))
	}
	success, ok := sig.Body[0].(bool)
	if !ok {
		return res, errors.Errorf("unable to convert 'success' from bool %v", sig.Body[0])
	}
	errStr, ok := sig.Body[1].(string)
	if !ok {
		return res, errors.Errorf("unable to convert 'errStr' from string %v", sig.Body[1])
	}
	res.success = success
	res.errStr = errStr
	return res, nil
}

// StartAndWaitForQuiescence starts the modemfwd job and waits for the initial sequence to complete
// or until an error is logged.
func StartAndWaitForQuiescence(ctx context.Context) error {
	watcher, err := WatchUpdateFirmwareCompleted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to watch for UpdateFirmwareCompleted")
	}
	defer watcher.Close(ctx)

	err = upstart.StartJob(ctx, JobName)
	if err != nil {
		return errors.Wrapf(err, "failed to start %q", JobName)
	}

	// Map D-Bus signals into UpdateFirmwareCompletedSignal.
	select {
	case sig := <-watcher.Signals:
		signal, err := parseUpdateFirmwareCompletedSignal(sig)
		if err != nil {
			return errors.Wrap(err, "signal returned error")
		}
		if signal.errStr != "" {
			return errors.New("modemfwd started with failure: " + signal.errStr)
		}
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "didn't get UpdateFirmwareCompleted D-Bus signal")
	}
}

// WatchUpdateFirmwareCompleted returns a SignalWatcher to observe the
// "UpdateFirmwareCompleted" signal.
func WatchUpdateFirmwareCompleted(ctx context.Context) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "UpdateFirmwareCompleted",
	}
	return dbusutil.NewSignalWatcherForSystemBus(ctx, spec)
}
