// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/caller"
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
// Users can set m.Debug = true to see verbose logging for developing/debugging purpose.
type Manager struct {
	dbusObject *DBusObject
	props      *Properties
	Debug      bool // Debug set to enable verbose logging. Default false.
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
	conn, obj, err := dbusutil.Connect(ctx, dbusService, dbusManagerPath)
	if err != nil {
		return nil, err
	}
	dbusObj := &DBusObject{iface: dbusManagerInterface, obj: obj, conn: conn}
	props, err := NewProperties(ctx, dbusObj)
	if err != nil {
		return nil, err
	}
	return &Manager{dbusObject: dbusObj, props: props, Debug: false}, nil
}

// Properties returns existing properties without refreshing.
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
	if err == nil {
		m.props = props
	}
	// Note that we don't emit props here because it is too verbose.
	m.logReturn(ctx, err)
	return props, err
}

// FindMatchingService returns a service with matching properties.
// Note that it also refreshes properties.
func (m *Manager) FindMatchingService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	p, err := m.findMatchingServiceInner(ctx, props, false)
	m.logReturn(ctx, err, p)
	return p, err
}

// FindMatchingAnyService returns any service including not visible with matching properties.
// Note that it also refreshes properties.
func (m *Manager) FindMatchingAnyService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	p, err := m.findMatchingServiceInner(ctx, props, true)
	m.logReturn(ctx, err, p)
	return p, err
}

// getObjectPaths returns a non-empty list of dbus.ObjectPath of the given property.
// If there's no ObjectPath of the property, it will wait for property change till timeout.
// Note that it also refreshes properties.
func (m *Manager) getObjectPaths(ctx context.Context, prop string) ([]dbus.ObjectPath, error) {
	pw, err := m.Properties().CreateWatcher(ctx)
	if err != nil {
		return nil, err
	}
	defer pw.Close(ctx)

	for {
		props, err := m.GetProperties(ctx)
		if err != nil {
			return nil, err
		}
		paths, err := props.GetObjectPaths(prop)
		if err != nil {
			return nil, err
		}
		if len(paths) > 0 {
			return paths, nil
		}
		if err := pw.WaitAll(ctx, prop); err != nil {
			return nil, err
		}
	}
}

// getServices obtains a list of service paths of the manager.
// If complete is set, also obtains hidden service paths.
// Note that it also refreshes properties.
func (m *Manager) getServices(ctx context.Context, complete bool) ([]dbus.ObjectPath, error) {
	name := ManagerPropertyServices
	if complete {
		name = ManagerPropertyServiceCompleteList
	}
	ps, err := m.getObjectPaths(ctx, name)
	m.logReturn(ctx, err, ps)
	return ps, err
}

// findMatchingServiceInner is the implementation of FindMatchingService and FindMatchingAnyService.
func (m *Manager) findMatchingServiceInner(ctx context.Context, props map[string]interface{}, complete bool) (dbus.ObjectPath, error) {
	paths, err := m.getServices(ctx, complete)
	if err != nil {
		return "", err
	}

ForServicePaths:
	for _, path := range paths {
		service, err := NewService(ctx, path)
		if err != nil {
			return "", err
		}
		sp := service.Properties()

		for k, expect := range props {
			if actual, err := sp.Get(k); err != nil || !reflect.DeepEqual(expect, actual) {
				continue ForServicePaths
			}
		}
		return path, nil
	}
	err = errors.New("unable to find matching service")
	return "", err
}

// WaitForServiceProperties polls FindMatchingService() for a service matching the given properties.
func (m *Manager) WaitForServiceProperties(ctx context.Context, props map[string]interface{}, timeout time.Duration) error {
	err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := m.FindMatchingService(ctx, props)
		return err
	}, &testing.PollOptions{Timeout: timeout})
	m.logReturn(ctx, err)
	return err
}

// GetProfiles returns a list of profiles.
// Note that it also refreshes properties.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	ps, err := m.getObjectPaths(ctx, ManagerPropertyProfiles)
	m.logReturn(ctx, err, ps)
	return ps, err
}

// GetDevices returns a list of devices.
// Note that it also refreshes properties.
func (m *Manager) GetDevices(ctx context.Context) ([]dbus.ObjectPath, error) {
	ps, err := m.getObjectPaths(ctx, ManagerPropertyDevices)
	m.logReturn(ctx, err, ps)
	return ps, err
}

// ConfigureService configures a service with the given properties.
func (m *Manager) ConfigureService(ctx context.Context, props map[string]interface{}) error {
	err := m.dbusObject.Call(ctx, "ConfigureService", props).Err
	m.logReturn(ctx, err)
	return err
}

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	var service dbus.ObjectPath
	err := m.dbusObject.Call(ctx, "ConfigureServiceForProfile", path, props).Store(&service)
	if err != nil {
		err = errors.Wrap(err, "failed to configure service")
	}
	m.logReturn(ctx, err, service)
	return service, err
}

// CreateProfile creates a profile.
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	err := m.dbusObject.Call(ctx, "CreateProfile", name).Store(&profile)
	if err != nil {
		err = errors.Wrap(err, "failed to create profile")
	}
	m.logReturn(ctx, err, profile)
	return profile, err
}

// PushProfile pushes a profile.
func (m *Manager) PushProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	var profile dbus.ObjectPath
	err := m.dbusObject.Call(ctx, "PushProfile", name).Store(&profile)
	if err != nil {
		err = errors.Wrap(err, "failed to create profile")
	}
	m.logReturn(ctx, err, profile)
	return profile, err
}

// RemoveProfile removes the profile with the given name.
func (m *Manager) RemoveProfile(ctx context.Context, name string) error {
	err := m.dbusObject.Call(ctx, "RemoveProfile", name).Err
	m.logReturn(ctx, err)
	return err
}

// PopProfile pops the profile with the given name if it is on top of the stack.
func (m *Manager) PopProfile(ctx context.Context, name string) error {
	err := m.dbusObject.Call(ctx, "PopProfile", name).Err
	m.logReturn(ctx, err)
	return err
}

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	err := m.dbusObject.Call(ctx, "PopAllUserProfiles").Err
	m.logReturn(ctx, err)
	return err
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology Technology) error {
	err := m.dbusObject.Call(ctx, "EnableTechnology", string(technology)).Err
	m.logReturn(ctx, err)
	return err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology Technology) error {
	err := m.dbusObject.Call(ctx, "DisableTechnology", string(technology)).Err
	m.logReturn(ctx, err)
	return err
}

// GetDevicesByTechnology returns list of Devices of the specified technology.
// Note that it also refreshes properties.
func (m *Manager) GetDevicesByTechnology(ctx context.Context, technology Technology) (ret []*Device, err error) {
	tech := string(technology)
	defer func() {
		m.logReturnDefer(ctx, err, ret)
	}()

	// Refresh properties first.
	_, err = m.GetProperties(ctx)
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
		} else if devType != tech {
			continue
		}
		ret = append(ret, dev)
	}
	return ret, nil
}

// logReturnName logs function's return status if m.Debug is set.
// Note that function name is specified.
func (m *Manager) logReturnName(ctx context.Context, name string, err error, rets ...interface{}) {
	if !m.Debug {
		return
	}

	if err != nil {
		testing.ContextLogf(ctx, "%s failed: %s", name, err)
	} else if len(rets) == 0 {
		testing.ContextLogf(ctx, "%s success", name)
	} else if len(rets) == 1 {
		testing.ContextLogf(ctx, "%s returns %v", name, rets[0])
	} else {
		var rs []string
		for _, r := range rets {
			rs = append(rs, fmt.Sprint(r))
		}
		testing.ContextLogf(ctx, "%s returns (%v)", name, strings.Join(rs, ", "))
	}
}

// logReturn logs its caller's return status if m.Debug is set.
// Note that the caller's name is derived from call stack.
func (m *Manager) logReturn(ctx context.Context, err error, rets ...interface{}) {
	m.logReturnName(ctx, path.Base(caller.Get(2)), err, rets...)
}

// logReturnDefer logs the caller's return status if m.Debug is set.
// It is used in a function's deferred call.
func (m *Manager) logReturnDefer(ctx context.Context, err error, rets ...interface{}) {
	m.logReturnName(ctx, path.Base(caller.Get(3)), err, rets...)
}
