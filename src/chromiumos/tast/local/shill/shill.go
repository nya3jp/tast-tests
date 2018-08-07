// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides DBus wrappers and utilities for shill service.
package shill

import (
	"context"
	"fmt"
	"os"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"

	"github.com/godbus/dbus"
)

const (
	startLockPath = "/run/lock/shill-start.lock"

	dbusService          = "org.chromium.flimflam"
	dbusPath             = "/" // crosbug.com/20135
	dbusManagerInterface = "org.chromium.flimflam.Manager"
)

// acquireStartLock acquires the start lock of shill. Holding the lock prevents recover_duts from
// restarting shill even if network connection is down.
func acquireStartLock() error {
	// We assume that there is no concurrent process trying to create/delete the lock file,
	// but check them just in case. This must not happen.
	if _, err := os.Stat(startLockPath); err == nil {
		p, _ := os.Readlink(startLockPath)
		return fmt.Errorf("shill start lock is held by another process: %s", p)
	}

	// Remove an obsolete lock file if it exists.
	os.Remove(startLockPath)

	// Create the lock file.
	if err := os.Symlink(fmt.Sprintf("/proc/%d", os.Getpid()), startLockPath); err != nil {
		return fmt.Errorf("failed creating a lock file: %v", err)
	}
	return nil
}

// releaseStartLock releases the start lock of shill.
func releaseStartLock() error {
	if err := os.Remove(startLockPath); err != nil {
		return fmt.Errorf("failed deleting a lock file: %v", err)
	}
	return nil
}

// SafeStop stops the shill service temporarily.
// This function does not only call upstart.StopJob, but also ensures shill is not started by
// recover_duts.
// As soon as you are done, SafeStart must be called to recover network connectivity.
func SafeStop(ctx context.Context) error {
	if err := acquireStartLock(); err != nil {
		return err
	}
	return upstart.StopJob(ctx, "shill")
}

// SafeStart starts the shill service.
func SafeStart(ctx context.Context) error {
	defer releaseStartLock()
	return upstart.RestartJob(ctx, "shill")
}

// Manager wraps a DBus object of Manager in shill.
type Manager struct {
	obj dbus.BusObject
}

// NewManager connects to shill via DBus and creates Manager object.
func NewManager(ctx context.Context) (*Manager, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed connecting to system bus: %v", err)
	}

	if err := dbusutil.WaitForService(ctx, conn, dbusService); err != nil {
		return nil, fmt.Errorf("failed waiting for shill service: %v", err)
	}

	obj := conn.Object(dbusService, dbusPath)
	return &Manager{obj}, nil
}

// GetProfiles returns a list of profiles.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	props, err := m.getProperties(ctx)
	if err != nil {
		return nil, err
	}
	return props["Profiles"].([]dbus.ObjectPath), nil
}

// TemporaryProfile pops all non-default profiles and pushes a temporary profile for testing.
// After testing is done, the temporary profile must be deleted by calling closer.
func (m *Manager) TemporaryProfile(ctx context.Context) (closer func(context.Context) error, err error) {
	const name = "test"

	closer = func(context.Context) error { return nil }

	if err := m.call(ctx, "PopAllUserProfiles").Err; err != nil {
		return closer, fmt.Errorf("failed popping user profiles: %v", err)
	}

	m.call(ctx, "RemoveProfile")

	if err := m.call(ctx, "CreateProfile", name).Err; err != nil {
		return closer, fmt.Errorf("failed creating a test profile: %v", err)
	}

	if err := m.call(ctx, "PushProfile", name).Err; err != nil {
		return closer, fmt.Errorf("failed pushing a test profile: %v", err)
	}

	closer = func(ctx context.Context) error {
		err := m.call(ctx, "PopProfile", name).Err
		if rerr := m.call(ctx, "RemoveProfile", name).Err; err == nil {
			err = rerr
		}
		return err
	}
	return closer, nil
}

// ConfigureServices configures the service with params.
func (m *Manager) ConfigureService(ctx context.Context, params map[string]interface{}) error {
	return m.call(ctx, "ConfigureService", &params).Err
}

// getProperties returns a list of properties provided by Manager.
func (m *Manager) getProperties(ctx context.Context) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	if err := m.call(ctx, "GetProperties").Store(&props); err != nil {
		return nil, fmt.Errorf("failed getting properties: %v", err)
	}
	return props, nil
}

// call is a wrapper of dbus.BusObject.CallWithContext.
func (m *Manager) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return m.obj.CallWithContext(ctx, dbusManagerInterface+"."+method, 0, args...)
}
