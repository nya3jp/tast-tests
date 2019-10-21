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

// Manager wraps a Manager D-Bus object in shill.
type Manager struct {
	obj dbus.BusObject
}

// Technology is the type of a shill device's technology
type Technology string

// Device technologies
// Refer to Flimflam type options in
// https://chromium.googlesource.com/chromiumos/platform2/+/refs/heads/master/system_api/dbus/shill/dbus-constants.h#334
const (
	TechnologyBluetooth Technology = "bluetooth"
	TechnologyCellular  Technology = "cellular"
	TechnologyEthernet  Technology = "ethernet"
	TechnologyPPPoE     Technology = "pppoe"
	TechnologyVPN       Technology = "vpn"
	TechnologyWifi      Technology = "wifi"
)

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
func (m *Manager) FindMatchingService(ctx context.Context, props map[ServiceProperty]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceInner(ctx, props, false)
}

// FindMatchingAnyService returns any service including not visible with matching properties.
func (m *Manager) FindMatchingAnyService(ctx context.Context, props map[ServiceProperty]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceInner(ctx, props, true)
}

func (m *Manager) findMatchingServiceInner(ctx context.Context, props map[ServiceProperty]interface{}, complete bool) (dbus.ObjectPath, error) {
	managerProps, err := m.getProperties(ctx)
	if err != nil {
		return "", err
	}

	propName := "Services"
	if complete {
		propName = "ServiceCompleteList"
	}
	for _, path := range managerProps[propName].([]dbus.ObjectPath) {
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
func (m *Manager) WaitForServiceProperties(ctx context.Context, props map[ServiceProperty]interface{}, timeout time.Duration) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := m.FindMatchingService(ctx, props)
		return err
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return err
	}
	return nil
}

type managerProps map[string]interface{}

func (m managerProps) getDBusPaths(key string) ([]dbus.ObjectPath, error) {
	value, ok := m[key]
	if !ok {
		return nil, errors.Errorf("property is not present: %s", key)
	}
	arr, ok := value.([]dbus.ObjectPath)
	if !ok {
		return nil, errors.Errorf("can not convert value to []dbus.ObjectPath: %v", value)
	}
	return arr, nil
}

// getProperties returns a list of properties provided by the service.
func (m *Manager) getProperties(ctx context.Context) (managerProps, error) {
	props := make(managerProps)
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
	return props.getDBusPaths("Profiles")
}

// GetDevices returns a list of devices.
func (m *Manager) GetDevices(ctx context.Context) ([]dbus.ObjectPath, error) {
	props, err := m.getProperties(ctx)
	if err != nil {
		return nil, err
	}
	return props.getDBusPaths("Devices")
}

// ConfigureService configures a service with the given properties.
func (m *Manager) ConfigureService(ctx context.Context, props map[ServiceProperty]interface{}) error {
	return call(ctx, m.obj, dbusManagerInterface, "ConfigureService", props).Err
}

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[ServiceProperty]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := call(ctx, m.obj, dbusManagerInterface, "ConfigureServiceForProfile", path, props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// CreateProfile creates a profile.
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := call(ctx, m.obj, dbusManagerInterface, "CreateProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// PushProfile pushes a profile.
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

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return call(ctx, m.obj, dbusManagerInterface, "PopAllUserProfiles").Err
}

// EnableTechnology enables a technology interface.
<<<<<<< HEAD   (3a4a84 chrome: Ignore errors of rpcc.Conn.Close.)
func (m *Manager) EnableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "EnableTechnology", technology).Err
=======
func (m *Manager) EnableTechnology(ctx context.Context, technology Technology) error {
	return m.dbusObject.Call(ctx, "EnableTechnology", string(technology)).Err
>>>>>>> CHANGE (bba4c1 Tast: Poll the WiFi interface name until timeout.)
}

// DisableTechnology disables a technology interface.
<<<<<<< HEAD   (3a4a84 chrome: Ignore errors of rpcc.Conn.Close.)
func (m *Manager) DisableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "DisableTechnology", technology).Err
=======
func (m *Manager) DisableTechnology(ctx context.Context, technology Technology) error {
	return m.dbusObject.Call(ctx, "DisableTechnology", string(technology)).Err
}

// GetDevicesByTechnology returns list of Devices of the specified technology.
func (m *Manager) GetDevicesByTechnology(ctx context.Context, technology Technology) ([]*Device, error) {
	var devs []*Device
	// Refresh properties first.
	_, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	devPaths, err := m.GetDevices(ctx)
	if err != nil {
		return nil, err
	}

	for _, path := range devPaths {
		dev, err := NewDevice(ctx, path)
		// It is forgivable as a device may go down anytime.
		if err != nil {
			testing.ContextLogf(ctx, "Error getting a device %q: %v", path, err)
			continue
		}
		if devType, err := dev.Properties().GetString(DevicePropertyType); err != nil {
			testing.ContextLogf(ctx, "Error getting the type of the device %q: %v", path, err)
			continue
		} else if devType != string(technology) {
			continue
		}
		devs = append(devs, dev)
	}
	return devs, nil
>>>>>>> CHANGE (bba4c1 Tast: Poll the WiFi interface name until timeout.)
}
