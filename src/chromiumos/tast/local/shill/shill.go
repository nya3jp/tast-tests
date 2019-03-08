// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"
	"fmt"
	"os"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

const (
	startLockPath = "/run/lock/shill-start.lock"

	dbusService          = "org.chromium.flimflam"
	dbusManagerPath      = "/" // crosbug.com/20135
	dbusManagerInterface = "org.chromium.flimflam.Manager"
	dbusServiceInterface = "org.chromium.flimflam.Service"
)

// acquireStartLock acquires the start lock of shill. Holding the lock prevents recover_duts from
// restarting shill (crbug.com/473976#c9).
func acquireStartLock() error {
	// We assume that there is no concurrent process trying to create/delete the lock file,
	// but check them just in case. This must not happen.
	if _, err := os.Stat(startLockPath); err == nil {
		p, _ := os.Readlink(startLockPath)
		return errors.Errorf("shill start lock is held by another process: %s", p)
	}

	// Remove an obsolete lock file if it exists.
	os.Remove(startLockPath)

	// Create the lock file. We set the link destination to our proc entry so that the lock is
	// automatically released even if the process crashes.
	if err := os.Symlink(fmt.Sprintf("/proc/%d", os.Getpid()), startLockPath); err != nil {
		return errors.Wrap(err, "failed creating a lock file")
	}
	return nil
}

// releaseStartLock releases the start lock of shill.
func releaseStartLock() error {
	if err := os.Remove(startLockPath); err != nil {
		return errors.Wrap(err, "failed deleting a lock file")
	}
	return nil
}

// SafeStop stops the shill service temporarily.
// This function does not only call upstart.StopJob, but also ensures shill is not started by
// recover_duts (crbug.com/473976#c9). Remember to call SafeStart once you are done.
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

// D-Bus object type definitions

// Object wraps a D-Bus object and its interface
type Object struct {
	dbusInterface string
	obj           dbus.BusObject
}

// Manager D-Bus object
type Manager struct {
	Object
}

// Service D-Bus object
type Service struct {
	Object
}

// Instantiate D-Bus objects

// NewObject connects to shill via the specified D-Bus path
func NewObject(ctx context.Context, path dbus.ObjectPath) (*Object, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	return &Object{obj: obj}, nil
}

// NewManager connects to shill's Manager
func NewManager(ctx context.Context) (*Manager, error) {
	obj, err := NewObject(ctx, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	m := &Manager{*obj}
	m.dbusInterface = dbusManagerInterface
	return m, nil
}

// NewService connects to the service at the given service path.
func NewService(ctx context.Context, path dbus.ObjectPath) (*Service, error) {
	obj, err := NewObject(ctx, path)
	if err != nil {
		return nil, err
	}
	s := &Service{*obj}
	s.dbusInterface = dbusServiceInterface
	return s, nil
}

// Generic methods for all objects

// call is a wrapper of dbus.BusObject.CallWithContext.
func (s *Object) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return s.obj.CallWithContext(ctx, s.dbusInterface+"."+method, 0, args...)
}

// getProperties returns a list of properties provided by the object.
func (s *Object) getProperties(ctx context.Context) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	if err := s.call(ctx, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// GetProfiles returns a list of profiles.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	props, err := m.getProperties(ctx)
	if err != nil {
		return nil, err
	}
	return props["Profiles"].([]dbus.ObjectPath), nil
}

// Methods specific to Manager object

// ConfigureServiceForProfile configures a service at the given profile path
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := m.call(ctx, "ConfigureServiceForProfile", path, props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// FindMatchingService returns a service with matching properties
func (m *Manager) FindMatchingService(ctx context.Context, props map[string]interface{}) (*Service, error) {
	managerProps, err := m.getProperties(ctx)
	if err != nil {
		return nil, err
	}

	for _, path := range managerProps["Services"].([]dbus.ObjectPath) {
		s, err := NewService(ctx, path)
		if err != nil {
			return nil, err
		}
		serviceProps, err := s.getProperties(ctx)
		if err != nil {
			return nil, err
		}

		match := true
		for key, val1 := range props {
			if val2, ok := serviceProps[key]; !ok || val1 != val2 {
				match = false
				break
			}
		}
		if match {
			return s, nil
		}
	}
	return nil, errors.New("Unable to find matching service")
}
