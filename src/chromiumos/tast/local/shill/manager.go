// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
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
	m.logReturn(ctx, "GetProperties()", err)
	return props, err
}

// findMatchingServiceWrapper is a wrapper of findMatchingServiceInner with debug logging and error wrapping.
func (m *Manager) findMatchingServiceWrapper(ctx context.Context, props map[string]interface{}, complete bool, name string) (dbus.ObjectPath, error) {
	ctx, st := timing.Start(ctx, "manager."+name)
	defer st.End()

	p, err := m.findMatchingServiceInner(ctx, props, complete)
	if err != nil {
		err = errors.Wrapf(err, "manager.%s() failed", name)
	}
	m.logReturn(ctx, name+"()", err, p)
	return p, err
}

// FindMatchingService returns a service with matching properties.
// Note that it also refreshes properties.
func (m *Manager) FindMatchingService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceWrapper(ctx, props, false, "FindMatchingService")
}

// FindMatchingAnyService returns any service including not visible with matching properties.
// Note that it also refreshes properties.
func (m *Manager) FindMatchingAnyService(ctx context.Context, props map[string]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceWrapper(ctx, props, true, "FindMatchingAnyService")
}

// getObjectPaths returns a non-empty list of dbus.ObjectPath of the given property.
// method is the name of function who calls it, which is used for logging.
// If there's no ObjectPath of the property, it will wait for property change till timeout.
// Note that it also refreshes properties.
func (m *Manager) getObjectPaths(ctx context.Context, prop, method string) ([]dbus.ObjectPath, error) {
	method = method + "()"
	pw, err := m.Properties().CreateWatcher(ctx)
	if err != nil {
		m.logReturn(ctx, method, err)
		return nil, err
	}
	defer pw.Close(ctx)

	for {
		props, err := m.GetProperties(ctx)
		if err != nil {
			m.logReturn(ctx, method, err)
			return nil, err
		}
		paths, err := props.GetObjectPaths(prop)
		if err != nil {
			m.logReturn(ctx, method, err)
			return nil, err
		}
		if len(paths) > 0 {
			m.logReturn(ctx, method, err, paths)
			return paths, nil
		}
		if err := pw.WaitAll(ctx, prop); err != nil {
			m.logReturn(ctx, method, err)
			return nil, err
		}
	}
}

// getServices obtains a list of service paths of the manager.
// If complete is set, also obtains hidden service paths.
// Note that it also refreshes properties.
func (m *Manager) getServices(ctx context.Context, complete bool) ([]dbus.ObjectPath, error) {
	if complete {
		return m.getObjectPaths(ctx, ManagerPropertyServiceCompleteList, "getCompleteServices")
	}
	return m.getObjectPaths(ctx, ManagerPropertyServices, "getServices")
}

// GetProfiles returns a list of profiles.
// Note that it also refreshes properties.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	return m.getObjectPaths(ctx, ManagerPropertyProfiles, "GetProfiles")
}

// GetDevices returns a list of devices.
// Note that it also refreshes properties.
func (m *Manager) GetDevices(ctx context.Context) ([]dbus.ObjectPath, error) {
	return m.getObjectPaths(ctx, ManagerPropertyDevices, "GetDevices")
}

// findMatchingServiceInner is the implementation of FindMatchingService and FindMatchingAnyService.
// Do not call this function directly.
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
	ctx, st := timing.Start(ctx, "manager.WaitForServiceProperties")
	defer st.End()

	err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := m.FindMatchingService(ctx, props)
		return err
	}, &testing.PollOptions{Timeout: timeout})
	if err != nil {
		err = errors.Wrap(err, "manager.WaitForServiceProperties() failed")
	}
	m.logReturn(ctx, "WaitForServiceProperties()", err)
	return err
}

// call calls m.dbusObject.Call() and returns checked error.
// method is the name of function who calls it, which is used for logging.
// Also performs debug logging.
func (m *Manager) call(ctx context.Context, method string, args ...interface{}) error {
	err := m.dbusObject.Call(ctx, method, args...).Err
	m.logReturn(ctx, method+"()", err)
	return err
}

// ConfigureService configures a service with the given properties.
func (m *Manager) ConfigureService(ctx context.Context, props map[string]interface{}) error {
	return m.call(ctx, "ConfigureService", props)
}

// callStorePath calls m.dbusObject.Call() and returns ObjectPath.
// It also performs debug logging.
func (m *Manager) callStorePath(ctx context.Context, method string, args ...interface{}) (dbus.ObjectPath, error) {
	var path dbus.ObjectPath
	err := m.dbusObject.Call(ctx, method, args...).Store(&path)
	if err != nil {
		err = errors.Wrapf(err, "manager.%s() failed", method)
	}
	m.logReturn(ctx, method+"()", err, path)
	return path, nil
}

// ConfigureServiceForProfile configures a service at the given profile path.
func (m *Manager) ConfigureServiceForProfile(ctx context.Context, path dbus.ObjectPath, props map[string]interface{}) (dbus.ObjectPath, error) {
	return m.callStorePath(ctx, "ConfigureServiceForProfile", path, props)
}

// CreateProfile creates a profile.
func (m *Manager) CreateProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	return m.callStorePath(ctx, "CreateProfile", name)
}

// PushProfile pushes a profile.
func (m *Manager) PushProfile(ctx context.Context, name string) (dbus.ObjectPath, error) {
	return m.callStorePath(ctx, "PushProfile", name)
}

// RemoveProfile removes the profile with the given name.
func (m *Manager) RemoveProfile(ctx context.Context, name string) error {
	return m.call(ctx, "RemoveProfile", name)
}

// PopProfile pops the profile with the given name if it is on top of the stack.
func (m *Manager) PopProfile(ctx context.Context, name string) error {
	return m.call(ctx, "PopProfile", name)
}

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return m.call(ctx, "PopAllUserProfiles")
}

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology Technology) error {
	return m.call(ctx, "EnableTechnology", string(technology))
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology Technology) error {
	return m.call(ctx, "DisableTechnology", string(technology))
}

// GetDevicesByTechnology returns list of Devices of the specified technology.
// Note that it also refreshes properties.
func (m *Manager) GetDevicesByTechnology(ctx context.Context, technology Technology) (ret []*Device, err error) {
	tech := string(technology)
	defer func() {
		m.logReturn(ctx, "GetDevicesByTechnology("+tech+")", err, ret)
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

// logReturn logs function return status if m.Debug is set.
func (m *Manager) logReturn(ctx context.Context, method string, err error, rets ...interface{}) {
	if m.Debug {
		if err != nil {
			testing.ContextLogf(ctx, "manager.%s failed: %s", method, err)
		} else if len(rets) == 0 {
			testing.ContextLogf(ctx, "manager.%s success", method)
		} else if len(rets) == 1 {
			testing.ContextLogf(ctx, "manager.%s returns %v", method, rets[0])
		} else {
			var rs []string
			for _, r := range rets {
				rs = append(rs, fmt.Sprint(r))
			}
			testing.ContextLogf(ctx, "mamager.%s returns (%v)", method, strings.Join(rs, ", "))
		}
	}
}
