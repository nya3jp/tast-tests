// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

// Manager property names.
const (
	ManagerPropertyDevices             = "Devices"
	ManagerPropertyProfiles            = "Profiles"
	ManagerPropertyServices            = "Services"
	ManagerPropertyServiceCompleteList = "ServiceCompleteList"
)

// Manager wraps a Manager D-Bus object in shill.
type Manager struct {
	dbusObject *DBusObject
	props      *Properties
}

// NewManager connects to shill's Manager.
func NewManager(ctx context.Context) (*Manager, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusService, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	dbusObj := &DBusObject{iface: dbusManagerInterface, obj: obj, conn: conn}
	props, err := NewProperties(ctx, dbusObj)
	if err != nil {
		return nil, err
	}
	return &Manager{dbusObject: dbusObj, props: props}, nil
}

// Properties returns existing properties.
func (m *Manager) Properties() *Properties {
	return m.props
}

// String returns the path of the manager.
// It is so named to conform to the Stringer interface.
func (m *Manager) String() string {
	return m.dbusObject.String()
}

// GetProperties refreshes and returns properties.
func (m *Manager) GetProperties(ctx context.Context) (*Properties, error) {
	props, err := NewProperties(ctx, m.dbusObject)
	if err != nil {
		return nil, err
	}
	m.props = props
	return props, nil
}

// FindMatchingService returns a service with matching properties.
func (m *Manager) FindMatchingService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceInner(ctx, props, false)
}

// FindMatchingAnyService returns any service including not visible with matching properties.
func (m *Manager) FindMatchingAnyService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceInner(ctx, props, true)
}

// getServicePaths obtains a list of service paths of the manager.
// If there's no service path, it'll wait until it is updated.
func (m *Manager) getServicePaths(ctx context.Context, complete bool) ([]dbus.ObjectPath, error) {
	serviceListName := ManagerPropertyServices
	if complete {
		serviceListName = ManagerPropertyServiceCompleteList
	}

	if _, err := m.GetProperties(ctx); err != nil {
		return nil, err
	}
	servicePaths, err := m.props.GetObjectPaths(serviceListName)
	if err != nil {
		return nil, err
	}
	if len(servicePaths) > 0 {
		return servicePaths, nil
	}

	pw, err := m.props.CreateWatcher(ctx)
	if err != nil {
		return nil, err
	}
	defer pw.Close(ctx)

	for {
		if err := pw.WaitAll(ctx, serviceListName); err != nil {
			return nil, err
		}
		if servicePaths, err := m.props.GetObjectPaths(serviceListName); err != nil {
			return nil, err
		} else if len(servicePaths) > 0 {
			return servicePaths, nil
		}
	}
}

func (m *Manager) findMatchingServiceInner(ctx context.Context, props map[string]interface{}, complete bool) (dbus.ObjectPath, error) {
	servicePaths, err := m.getServicePaths(ctx, complete)
	if err != nil {
		return "", err
	}

ForServicePaths:
	for _, path := range servicePaths {
		service, err := NewService(ctx, path)
		if err != nil {
			return "", err
		}
		serviceProps := service.Properties()

		for key, val1 := range props {
			if val2, err := serviceProps.Get(key); err != nil || !reflect.DeepEqual(val1, val2) {
				continue ForServicePaths
			}
		}
		return path, nil
	}
	return "", errors.New("unable to find matching service")
}

// WaitForServiceProperties polls FindMatchingService() for a service matching
// the given properties.
func (m *Manager) WaitForServiceProperties(ctx context.Context, props map[string]interface{}, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := m.FindMatchingService(ctx, props)
		return err
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return err
	}
	return nil
}

// GetProfiles returns a list of profiles.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	return m.props.GetObjectPaths(ManagerPropertyProfiles)
}

// GetDevices returns a list of devices.
func (m *Manager) GetDevices(ctx context.Context) ([]dbus.ObjectPath, error) {
	return m.props.GetObjectPaths(ManagerPropertyDevices)
}

// ConfigureService configures a service with the given properties.
func (m *Manager) ConfigureService(ctx context.Context, props map[string]interface{}) error {
	return m.dbusObject.Call(ctx, "ConfigureService", props).Err
}

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := m.dbusObject.Call(ctx, "ConfigureServiceForProfile", path, props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// CreateProfile creates a profile.
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := m.dbusObject.Call(ctx, "CreateProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// PushProfile pushes a profile.
func (m *Manager) PushProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := m.dbusObject.Call(ctx, "PushProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// RemoveProfile removes the profile with the given name.
func (m *Manager) RemoveProfile(ctx context.Context, name string) error {
	return m.dbusObject.Call(ctx, "RemoveProfile", name).Err
}

// PopProfile pops the profile with the given name if it is on top of the stack.
func (m *Manager) PopProfile(ctx context.Context, name string) error {
	return m.dbusObject.Call(ctx, "PopProfile", name).Err
}

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return m.dbusObject.Call(ctx, "PopAllUserProfiles").Err
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology string) error {
	return m.dbusObject.Call(ctx, "EnableTechnology", technology).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology string) error {
	return m.dbusObject.Call(ctx, "DisableTechnology", technology).Err
}
