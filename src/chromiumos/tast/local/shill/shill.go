// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	startLockPath = "/run/lock/shill-start.lock"

	dbusService          = "org.chromium.flimflam"
	dbusManagerPath      = "/" // crosbug.com/20135
	dbusDeviceInterface  = "org.chromium.flimflam.Device"
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
	managerProps, err := GetProperties(ctx, m.obj, dbusManagerInterface)
	if err != nil {
		return "", err
	}

	for _, path := range managerProps["Services"].([]dbus.ObjectPath) {
		serviceProps, err := GetPropsForService(ctx, path)
		if err != nil {
			return "", err
		}

		match := true
		for key, val1 := range props {
			if val2, ok := serviceProps[key]; !ok || val1 != val2 {
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

func GetPropsForService(ctx context.Context, path dbus.ObjectPath) (map[string]interface{}, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	return GetProperties(ctx, obj, dbusServiceInterface)
}

// call is a wrapper of dbus.BusObject.CallWithContext.
func call(ctx context.Context, obj dbus.BusObject, dbusInterface, method string, args ...interface{}) *dbus.Call {
	return obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// GetProperties returns a list of properties provided by the object.
func GetProperties(ctx context.Context, obj dbus.BusObject, dbusInterface string) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	if err := call(ctx, obj, dbusInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// GetProfiles returns a list of profiles.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	props, err := GetProperties(ctx, m.obj, dbusManagerInterface)
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

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "EnableTechnology", technology).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "DisableTechnology", technology).Err
}

// GetDevicesProperties gets the properties for each device listed on the manager.
func (m *Manager) GetDevicesProperties(ctx context.Context) ([]map[string]interface{}, error) {
	managerProps, err := GetProperties(ctx, m.obj, dbusManagerInterface)
	if err != nil {
		return nil, err
	}
	var toRet []map[string]interface{}
	devPaths := managerProps["Devices"].([]dbus.ObjectPath)
	for _, devPath := range devPaths {
		_, obj, err := dbusutil.Connect(ctx, dbusService, devPath)
		if err != nil {
			return nil, err
		}
		prop, err := GetProperties(ctx, obj, dbusDeviceInterface)
		if err != nil {
			return nil, err
		}
		toRet = append(toRet, prop)
	}
	return toRet, nil
}

// RequestScan tells shill to request a network scan on a specified interface.
func (m *Manager) RequestScan(ctx context.Context, props interface{}) error {
	return call(ctx, m.obj, dbusManagerInterface, "RequestScan", props).Err
}

// Connect will connect a manager to a service.
func (m *Manager) Connect(ctx context.Context, path dbus.ObjectPath) error {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return err
	}
	if err := call(ctx, obj, dbusServiceInterface, "Connect").Err; err != nil {
		return errors.Wrap(err, "failed to connect")
	}
	return nil
}

// Disconnect will disconnect a manager from a service.
func (m *Manager) Disconnect(ctx context.Context, path dbus.ObjectPath) error {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return err
	}
	if err := call(ctx, obj, dbusServiceInterface, "Disconnect").Err; err != nil {
		return errors.Wrap(err, "failed to disconnect")
	}
	return nil
}

// ConnectToWifiNetwork connects a flimflam manager to a wireless network
// that adheres to the requested properties.
func (m *Manager) ConnectToWifiNetwork(ctx context.Context, props map[string]interface{}) error {
	var servicePath dbus.ObjectPath
	var p map[string]interface{}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		path, err := m.FindMatchingService(ctx, props)
		if err != nil {
			return errors.Wrap(err, "could not find matching service")
		}
		p, err = GetPropsForService(ctx, path)
		if err != nil {
			return errors.Wrap(err, "could not get properties for service")
		}
		if err := m.RequestScan(ctx, "wifi"); err != nil {
			return errors.Wrap(err, "could not request scan on interface")
		}
		servicePath = path
		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to identify AP")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return m.Connect(ctx, servicePath)
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "couldn't connect to ap")
	}
	return nil
}

// DisconnectFromWifiNetwork disconnects a flimflam manager from a wireless network
// that adheres to the requested properties.
func (m *Manager) DisconnectFromWifiNetwork(ctx context.Context, props map[string]interface{}) error {
	var servicePath dbus.ObjectPath
	var p map[string]interface{}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		path, err := m.FindMatchingService(ctx, props)
		if err != nil {
			return errors.Wrap(err, "could not find matching service")
		}
		p, err = GetPropsForService(ctx, path)
		if err != nil {
			return errors.Wrap(err, "could not get properties for service")
		}
		if err := m.RequestScan(ctx, "wifi"); err != nil {
			return errors.Wrap(err, "could not request scan on interface")
		}
		servicePath = path
		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "failed to identify AP")
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return m.Disconnect(ctx, servicePath)
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrap(err, "couldn't connect to ap")
	}
	return nil
}
