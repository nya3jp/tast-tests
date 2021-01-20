// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	dbusManagerPath      = "/" // crosbug.com/20135
	dbusManagerInterface = "org.chromium.flimflam.Manager"
)

// Manager wraps a Manager D-Bus object in shill.
type Manager struct {
	*dbusutil.PropertyHolder
}

// Technology is the type of a shill device's technology
type Technology string

// Device technologies
// Refer to Flimflam type options in
// https://chromium.googlesource.com/chromiumos/platform2/+/refs/heads/main/system_api/dbus/shill/dbus-constants.h#334
const (
	TechnologyCellular Technology = shillconst.TypeCellular
	TechnologyEthernet Technology = shillconst.TypeEthernet
	TechnologyPPPoE    Technology = shillconst.TypePPPoE
	TechnologyVPN      Technology = shillconst.TypeVPN
	TechnologyWifi     Technology = shillconst.TypeWifi
)

// NewManager connects to shill's Manager.
func NewManager(ctx context.Context) (*Manager, error) {
	ph, err := dbusutil.NewPropertyHolder(ctx, dbusService, dbusManagerInterface, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	return &Manager{PropertyHolder: ph}, nil
}

// FindMatchingService returns the first Service that matches |expectProps|.
// If no matching Service is found, returns shillconst.ManagerFindMatchingServiceNotFound.
// Note that the complete list of Services is searched, including those with Visible=false.
// To find only visible services, please specify Visible=true in expectProps.
func (m *Manager) FindMatchingService(ctx context.Context, expectProps map[string]interface{}) (*Service, error) {
	ctx, st := timing.Start(ctx, "m.FindMatchingService")
	defer st.End()

	var servicePath dbus.ObjectPath
	if err := m.Call(ctx, "FindMatchingService", expectProps).Store(&servicePath); err != nil {
		return nil, err
	}
	service, err := NewService(ctx, servicePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to instantiate service %s", servicePath)
	}
	return service, nil
}

// WaitForServiceProperties returns the first matching Service who has the expected properties.
// If there's no matching service, it polls until timeout is reached.
// Noted that it searches all services including Visible=false ones. To focus on visible services,
// please specify Visible=true in expectProps.
func (m *Manager) WaitForServiceProperties(ctx context.Context, expectProps map[string]interface{}, timeout time.Duration) (*Service, error) {
	var service *Service
	if err := testing.Poll(ctx, func(ctx context.Context) (e error) {
		service, e = m.FindMatchingService(ctx, expectProps)
		return e
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return nil, err
	}
	return service, nil
}

// ProfilePaths returns a list of profile paths.
func (m *Manager) ProfilePaths(ctx context.Context) ([]dbus.ObjectPath, error) {
	p, err := m.GetProperties(ctx)
	if err != nil {
		return nil, err
	}
	return p.GetObjectPaths(shillconst.ManagerPropertyProfiles)
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
	path, err := props.GetObjectPath(shillconst.ManagerPropertyActiveProfile)
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
	paths, err := p.GetObjectPaths(shillconst.ManagerPropertyDevices)
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

// DeviceByType returns a device matching |type| or a "Device not found" error.
func (m *Manager) DeviceByType(ctx context.Context, deviceType string) (*Device, error) {
	devices, err := m.Devices(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		properties, err := d.GetProperties(ctx)
		if err != nil {
			return nil, err
		}
		t, err := properties.GetString(shillconst.DevicePropertyType)
		if err != nil {
			return nil, err
		}
		if t == deviceType {
			return d, nil
		}
	}
	return nil, errors.New("Device not found")
}

// ConfigureService configures a service with the given properties and returns its path.
func (m *Manager) ConfigureService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := m.Call(ctx, "ConfigureService", props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	if err := m.Call(ctx, "ConfigureServiceForProfile", path, props).Store(&service); err != nil {
		return "", errors.Wrap(err, "failed to configure service")
	}
	return service, nil
}

// CreateProfile creates a profile.
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := m.Call(ctx, "CreateProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// PushProfile pushes a profile.
func (m *Manager) PushProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	if err := m.Call(ctx, "PushProfile", name).Store(&profile); err != nil {
		return "", errors.Wrap(err, "failed to create profile")
	}
	return profile, nil
}

// RemoveProfile removes the profile with the given name.
func (m *Manager) RemoveProfile(ctx context.Context, name string) error {
	return m.Call(ctx, "RemoveProfile", name).Err
}

// PopProfile pops the profile with the given name if it is on top of the stack.
func (m *Manager) PopProfile(ctx context.Context, name string) error {
	return m.Call(ctx, "PopProfile", name).Err
}

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return m.Call(ctx, "PopAllUserProfiles").Err
}

// RequestScan requests a scan for the specified technology.
func (m *Manager) RequestScan(ctx context.Context, technology Technology) error {
	return m.Call(ctx, "RequestScan", string(technology)).Err
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology Technology) error {
	return m.Call(ctx, "EnableTechnology", string(technology)).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology Technology) error {
	return m.Call(ctx, "DisableTechnology", string(technology)).Err
}

func (m *Manager) hasTechnology(ctx context.Context, technologyProperty string, technology Technology) (bool, error) {
	prop, err := m.GetProperties(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get properties")
	}
	technologies, err := prop.GetStrings(technologyProperty)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get property: %s", technologyProperty)
	}
	for _, t := range technologies {
		if t == string(technology) {
			return true, nil
		}
	}
	return false, nil
}

// IsAvailable returns true if a technology is available.
func (m *Manager) IsAvailable(ctx context.Context, technology Technology) (bool, error) {
	return m.hasTechnology(ctx, shillconst.ManagerPropertyAvailableTechnologies, technology)
}

// IsEnabled returns true if a technology is enabled.
func (m *Manager) IsEnabled(ctx context.Context, technology Technology) (bool, error) {
	return m.hasTechnology(ctx, shillconst.ManagerPropertyEnabledTechnologies, technology)
}

// DevicesByTechnology returns list of Devices and their Properties snapshots of the specified technology.
func (m *Manager) DevicesByTechnology(ctx context.Context, technology Technology) ([]*Device, []*dbusutil.Properties, error) {
	var matches []*Device
	var props []*dbusutil.Properties

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
		if devType, err := p.GetString(shillconst.DevicePropertyType); err != nil {
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
		if devIface, err := p.GetString(shillconst.DevicePropertyInterface); err != nil {
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

		if _, err := pw.WaitAll(ctx, shillconst.ManagerPropertyDevices); err != nil {
			return nil, err
		}
	}
}
