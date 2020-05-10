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
	ManagerPropertyActiveProfile          = "ActiveProfile"
	ManagerPropertyDevices                = "Devices"
	ManagerPropertyEnabledTechnologies    = "EnabledTechnologies"
	ManagerPropertyProfiles               = "Profiles"
	ManagerPropertyProhibitedTechnologies = "ProhibitedTechnologies"
	ManagerPropertyServices               = "Services"
	ManagerPropertyServiceCompleteList    = "ServiceCompleteList"
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
	ph, err := NewPropertyHolder(ctx, dbusManagerInterface, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	return &Manager{PropertyHolder: ph}, nil
}

// findMatchingService returns a Service who has the expected properties.
// It first obtains a list of services (including hidden ones if complete is set).
// Then for each service, checks if it has the given props.
func (m *Manager) findMatchingService(ctx context.Context, expectProps map[string]interface{}, complete bool) (*Service, error) {
	serviceListName := ManagerPropertyServices
	if complete {
		serviceListName = ManagerPropertyServiceCompleteList
	}

	mProps, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	servicePaths, err := mProps.GetObjectPaths(serviceListName)
	if err != nil {
		return nil, err
	}
ForServicePaths:
	for _, path := range servicePaths {
		service, err := NewService(ctx, path)
		if err != nil {
			return nil, err
		}
		serviceProps, err := service.GetProperties(ctx)
		if err != nil {
			if dbusutil.IsDBusError(err, dbusutil.DBusErrorUnknownObject) {
				// This error is forgivable as a service may disappear anytime.
				continue
			}
			return nil, err
		}

		for key, val1 := range expectProps {
			if val2, err := serviceProps.Get(key); err != nil || !reflect.DeepEqual(val1, val2) {
				continue ForServicePaths
			}
		}
		return service, nil
	}
	return nil, errors.New("unable to find matching service")
}

// FindMatchingService returns a Service who has the expected properties.
func (m *Manager) FindMatchingService(ctx context.Context, expectProps map[string]interface{}) (*Service, error) {
	return m.findMatchingService(ctx, expectProps, false)
}

// FindAnyMatchingService returns a service who has the expected properties.
// It checks all services include hidden one.
func (m *Manager) FindAnyMatchingService(ctx context.Context, expectProps map[string]interface{}) (*Service, error) {
	return m.findMatchingService(ctx, expectProps, true)
}

// waitForServiceProperties returns a Service who has the expected properties.
// It also checks hidden services if complete is set.
// If there's no matching service, it polls until timeout is reached.
func (m *Manager) waitForServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration, complete bool) (*Service, error) {
	var service *Service
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		service, e = m.findMatchingService(ctx, expectProps, complete)
		return e
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, err
	}
	return service, nil
}

// WaitForServiceProperties returns a Service who has the expected properties.
// If there's no matching service, it polls until timeout is reached.
func (m *Manager) WaitForServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration) (*Service, error) {
	return m.waitForServiceProperties(ctx, expectProps, timeout, false)
}

// WaitForAnyServiceProperties returns a Service who has the expected properties.
// It checks all services, including hidden ones.
// If there's no matching service, it polls until timeout is reached.
func (m *Manager) WaitForAnyServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration) (*Service, error) {
	return m.waitForServiceProperties(ctx, expectProps, timeout, true)
}

// ProfilePaths returns a list of profile paths.
func (m *Manager) ProfilePaths(ctx context.Context) ([]dbus.ObjectPath, error) {
	p, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetObjectPaths(ManagerPropertyProfiles)
}

// Profiles returns a list of profiles.
func (m *Manager) Profiles(ctx context.Context) ([]*Profile, error) {
	paths, err := m.ProfilePaths(ctx)
	if err != nil {
		return nil, err
	}

	profiles := make([]*Profile, len(paths))
	for i, path := range paths {
		profiles[i], err = NewProfile(ctx, path)
		if err != nil {
			return nil, err
		}
	}
	return profiles, nil
}

// ActiveProfile returns the active profile.
func (m *Manager) ActiveProfile(ctx context.Context) (*Profile, error) {
	props, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	path, err := props.GetObjectPath(ManagerPropertyActiveProfile)
	if err != nil {
		return nil, err
	}
	return NewProfile(ctx, path)
}

// Devices returns a list of devices.
func (m *Manager) Devices(ctx context.Context) ([]*Device, error) {
	p, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	paths, err := p.GetObjectPaths(ManagerPropertyDevices)
	if err != nil {
		return nil, err
	}
	devs := make([]*Device, 0, len(paths))
	for _, path := range paths {
		d, err := NewDevice(ctx, path)
		if err != nil {
			return nil, err
		}
		devs = append(devs, d)
	}
	return devs, nil
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

// DevicesByTechnology returns list of Devices and their Properties snapshots of the specified technology.
func (m *Manager) DevicesByTechnology(ctx context.Context, technology Technology) ([]*Device, []*Properties, error) {
	var matches []*Device
	var props []*Properties

	devs, err := m.Devices(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, dev := range devs {
		p, err := dev.GetProperties(ctx)
		if err != nil {
			if dbusutil.IsDBusError(err, dbusutil.DBusErrorUnknownObject) {
				// This error is forgivable as a device may go down anytime.
				continue
			}
			return nil, nil, err
		}
		if devType, err := p.GetString(DevicePropertyType); err != nil {
			testing.ContextLogf(ctx, "Error getting the type of the device %q: %v", dev, err)
			continue
		} else if devType != string(technology) {
			continue
		}
		matches = append(matches, dev)
		props = append(props, p)
	}
	return matches, props, nil
}

// DeviceByName returns the Device matching the given interface name.
func (m *Manager) DeviceByName(ctx context.Context, iface string) (*Device, error) {
	devs, err := m.Devices(ctx)
	if err != nil {
		return nil, err
	}

	for _, dev := range devs {
		p, err := dev.GetProperties(ctx)
		if err != nil {
			if dbusutil.IsDBusError(err, dbusutil.DBusErrorUnknownObject) {
				// This error is forgivable as a device may go down anytime.
				continue
			}
			return nil, err
		}
		if devIface, err := p.GetString(DevicePropertyInterface); err != nil {
			testing.ContextLogf(ctx, "Error getting the device interface %q: %v", dev, err)
			continue
		} else if devIface == iface {
			return dev, nil
		}
	}
	return nil, errors.New("unable to find matching device")
}

// WaitForDeviceByName returns the Device matching the given interface name.
// If there's no match, it waits until one appears, or until timeout.
func (m *Manager) WaitForDeviceByName(ctx context.Context, iface string, timeout time.Duration) (*Device, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	pw, err := m.CreateWatcher(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a PropertiesWatcher")
	}
	defer pw.Close(ctx)

	for {
		if d, err := m.DeviceByName(ctx, iface); err == nil {
			return d, nil
		}

		if _, err := pw.WaitAll(ctx, ManagerPropertyDevices); err != nil {
			return nil, err
		}
	}
}
