// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"
	"reflect"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusManagerPath      = "/" // crosbug.com/20135
	dbusManagerInterface = "org.chromium.flimflam.Manager"
)

// Manager wraps a Manager D-Bus object in shill.
type Manager struct {
	obj dbus.BusObject
}

// NewManager connects to shill's Manager.
func NewManager(ctx context.Context) (*Manager, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	m := &Manager{obj: obj}
	return m, nil
}

// FindMatchingService returns a service with matching properties.
func (m *Manager) FindMatchingService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	managerProps, err := m.getProperties(ctx)
	if err != nil {
		return "", err
	}

	for _, path := range managerProps["Services"].([]dbus.ObjectPath) {
		service, err := NewService(ctx, path)
		if err != nil {
			return "", err
		}
		serviceProps, err := service.GetProperties(ctx)
		if err != nil {
			return "", err
		}

		match := true
		for key, val1 := range props {
			if val2, ok := serviceProps[key]; !ok || !reflect.DeepEqual(val1, val2) {
				match = false
				break
			}
		}
		if match {
			return path, nil
		}
	}
	return "", errors.New("unable to find matching service")
}

// WaitForServiceProperties polls FindMatchingService() for a service matching
// the given properties.
func (m *Manager) WaitForServiceProperties(ctx context.Context, props map[string]interface{}, timeoutSeconds time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := m.FindMatchingService(ctx, props)
		return err
	}, &testing.PollOptions{Timeout: timeoutSeconds * time.Second}); err != nil {
		return err
	}
	return nil
}

// getProperties returns a list of properties provided by the service.
func (m *Manager) getProperties(ctx context.Context) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	if err := call(ctx, m.obj, dbusManagerInterface, "GetProperties").Store(&props); err != nil {
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

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := call(ctx, m.obj, dbusManagerInterface, "ConfigureServiceForProfile", path, props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// CreateProfile creates a profile
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := call(ctx, m.obj, dbusManagerInterface, "CreateProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// PushProfile pushes a profile
func (m *Manager) PushProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := call(ctx, m.obj, dbusManagerInterface, "PushProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// RemoveProfile removes the profile with the given name.
func (m *Manager) RemoveProfile(ctx context.Context, name string) error {
	return call(ctx, m.obj, dbusManagerInterface, "RemoveProfile", name).Err
}

// PopProfile pops the profile with the given name if it is on top of the stack.
func (m *Manager) PopProfile(ctx context.Context, name string) error {
	return call(ctx, m.obj, dbusManagerInterface, "PopProfile", name).Err
}

// PopAllUserProfiles pops all user profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return call(ctx, m.obj, dbusManagerInterface, "PopAllUserProfiles").Err
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "EnableTechnology", technology).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "DisableTechnology", technology).Err
}
