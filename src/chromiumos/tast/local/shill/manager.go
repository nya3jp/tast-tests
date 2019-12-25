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
	PropertyHolder
}

// Technology is the type of a shill device's technology
type Technology string

// Device technologies
// Refer to Flimflam type options in
// https://chromium.googlesource.com/chromiumos/platform2/+/refs/heads/master/system_api/dbus/shill/dbus-constants.h#334
const (
	TechnologyCellular Technology = TypeCellular
	TechnologyEthernet Technology = TypeEthernet
	TechnologyPPPoE    Technology = TypePPPoE
	TechnologyVPN      Technology = TypeVPN
	TechnologyWifi     Technology = TypeWifi
)

// NewManager connects to shill's Manager.
func NewManager(ctx context.Context) (*Manager, error) {
	ph, err := NewPropertyHolder(ctx, dbusService, dbusManagerInterface, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	return &Manager{PropertyHolder: ph}, nil
}

// findMatchingService returns the path of a service who has the expected properties.
// It first obtains a list of services (including hidden ones if complete is set).
// Then for each service, checks if it has the given props.
func (m *Manager) findMatchingService(ctx context.Context, expectProps map[string]interface{}, complete bool) (dbus.ObjectPath, error) {
	serviceListName := ManagerPropertyServices
	if complete {
		serviceListName = ManagerPropertyServiceCompleteList
	}

	mProps, err := m.Properties(ctx)
	if err != nil {
		return "", err
	}
	servicePaths, err := mProps.GetObjectPaths(serviceListName)
	if err != nil {
		return "", err
	}
ForServicePaths:
	for _, path := range servicePaths {
		service, err := NewService(ctx, path)
		if err != nil {
			// TODO: log some?
			continue
		}
		serviceProps, err := service.Properties(ctx)
		if err != nil {
			// TODO: log some?
			continue
		}

		for key, val1 := range expectProps {
			if val2, err := serviceProps.Get(key); err != nil || !reflect.DeepEqual(val1, val2) {
				continue ForServicePaths
			}
		}
		return path, nil
	}
	return "", errors.New("unable to find matching service")
}

// FindMatchingService returns the path of a service who has the expect properties.
func (m *Manager) FindMatchingService(ctx context.Context, expectProps map[string]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingService(ctx, expectProps, false)
}

// FindAnyMatchingService returns the path of a service who has the expect properties.
// It checks all services include hidden one.
func (m *Manager) FindAnyMatchingService(ctx context.Context, expectProps map[string]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingService(ctx, expectProps, true)
}

// waitForServiceProperties returns the path of a service who has the expect properties.
// It also checks hidden services if complete is set.
// If there's no match service, it polls until timeout is reached.
func (m *Manager) waitForServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration, complete bool) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		path, e = m.findMatchingService(ctx, expectProps, complete)
		return e
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return "", err
	}
	return path, nil
}

// WaitForServiceProperties returns the path of a service who has the expect properties.
// If there's no match service, it polls until timeout is reached.
func (m *Manager) WaitForServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration) (dbus.ObjectPath, error) {
	return m.waitForServiceProperties(ctx, expectProps, timeout, false)
}

// WaitForAnyServiceProperties returns the path of a service who has the expect properties.
// It checks all services include hidden one.
// If there's no match service, it polls until timeout is reached.
func (m *Manager) WaitForAnyServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration) (dbus.ObjectPath, error) {
	return m.waitForServiceProperties(ctx, expectProps, timeout, true)
}

// GetProfiles returns a list of profiles.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	p, err := m.Properties(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetObjectPaths(ManagerPropertyProfiles)
}

// GetDevices returns a list of devices.
func (m *Manager) GetDevices(ctx context.Context) ([]dbus.ObjectPath, error) {
	p, err := m.Properties(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetObjectPaths(ManagerPropertyDevices)
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

// RequestScan requests a scan for the specified technology.
func (m *Manager) RequestScan(ctx context.Context, technology Technology) error {
	return m.dbusObject.Call(ctx, "RequestScan", string(technology)).Err
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology Technology) error {
	return m.dbusObject.Call(ctx, "EnableTechnology", string(technology)).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology Technology) error {
	return m.dbusObject.Call(ctx, "DisableTechnology", string(technology)).Err
}

// GetDevicesByTechnology returns list of Devices of the specified technology.
func (m *Manager) GetDevicesByTechnology(ctx context.Context, technology Technology) ([]*Device, []*Properties, error) {
	var devs []*Device
	var props []*Properties

	devPaths, err := m.GetDevices(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, path := range devPaths {
		dev, err := NewDevice(ctx, path)
		// It is forgivable as a device may go down anytime.
		if err != nil {
			testing.ContextLogf(ctx, "Error getting a device %q: %v", path, err)
			continue
		}
		p, err := dev.Properties(ctx)
		if err != nil {
			testing.ContextLogf(ctx, "Error getting properties of the device %q: %v", path, err)
		}
		if devType, err := p.GetString(DevicePropertyType); err != nil {
			testing.ContextLogf(ctx, "Error getting the type of the device %q: %v", path, err)
			continue
		} else if devType != string(technology) {
			continue
		}
		devs = append(devs, dev)
		props = append(props, p)
	}
	return devs, props, nil
}
