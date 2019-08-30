// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"
	"fmt"
	"os"
	"reflect"
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
	dbusManagerInterface = "org.chromium.flimflam.Manager"
	dbusServiceInterface = "org.chromium.flimflam.Service"

	// DefaultProfile is the path to the default profile shill creates.
	DefaultProfile = "/var/cache/shill/default.profile"
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
// This function waits until the default profile is created by shill (if it has been removed).
func SafeStart(ctx context.Context) error {
	defer releaseStartLock()
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		return err
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(DefaultProfile); os.IsNotExist(err) {
			return err
		} else if err != nil {
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second})
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

// ServiceProperty is the type for service property names.
type ServiceProperty string

// Property names defined in dbus-constants.h .
const (
	// Service property names.
	ServicePropertyName           ServiceProperty = "Name"
	ServicePropertyType           ServiceProperty = "Type"
	ServicePropertyMode           ServiceProperty = "Mode"
	ServicePropertySSID           ServiceProperty = "SSID"
	ServicePropertyStaticIPConfig ServiceProperty = "StaticIPConfig"
	ServicePropertySecurityClass  ServiceProperty = "SecurityClass"

	// WiFi service property names.
	ServicePropertyWiFiHiddenSSID ServiceProperty = "WiFi.HiddenSSID"

	// IPConfig property names.
	IPConfigPropertyNameServers = "NameServers"
)

// FindMatchingService returns a service with matching properties.
func (m *Manager) FindMatchingService(ctx context.Context, props map[ServiceProperty]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceInner(ctx, props, false)
}

// FindMatchingAnyService returns any service including not visible with matching properties.
func (m *Manager) FindMatchingAnyService(ctx context.Context, props map[ServiceProperty]interface{}) (dbus.ObjectPath, error) {
	return m.findMatchingServiceInner(ctx, props, true)
}

func (m *Manager) findMatchingServiceInner(ctx context.Context, props map[ServiceProperty]interface{}, complete bool) (dbus.ObjectPath, error) {
	managerProps, err := getProperties(ctx, m.obj, dbusManagerInterface)
	if err != nil {
		return "", err
	}

	propName := "Services"
	if complete {
		propName = "ServiceCompleteList"
	}
	for _, path := range managerProps[propName].([]dbus.ObjectPath) {
		serviceProps, err := getPropsForService(ctx, path)
		if err != nil {
			return "", err
		}

		match := true
		for key, val1 := range props {
			if val2, ok := serviceProps[string(key)]; !ok || !reflect.DeepEqual(val1, val2) {
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

func getPropsForService(ctx context.Context, path dbus.ObjectPath) (map[string]interface{}, error) {
	_, obj, err := dbusutil.Connect(ctx, dbusService, path)
	if err != nil {
		return nil, err
	}
	return getProperties(ctx, obj, dbusServiceInterface)
}

// call is a wrapper of dbus.BusObject.CallWithContext.
func call(ctx context.Context, obj dbus.BusObject, dbusInterface, method string, args ...interface{}) *dbus.Call {
	return obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// getProperties returns a list of properties provided by the object.
func getProperties(ctx context.Context, obj dbus.BusObject, dbusInterface string) (map[string]interface{}, error) {
	props := make(map[string]interface{})
	if err := call(ctx, obj, dbusInterface, "GetProperties").Store(&props); err != nil {
		return nil, errors.Wrap(err, "failed getting properties")
	}
	return props, nil
}

// GetProfiles returns a list of profiles.
func (m *Manager) GetProfiles(ctx context.Context) ([]dbus.ObjectPath, error) {
	props, err := getProperties(ctx, m.obj, dbusManagerInterface)
	if err != nil {
		return nil, err
	}
	return props["Profiles"].([]dbus.ObjectPath), nil
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

// EnableTechnology enables a technology interface.
func (m *Manager) EnableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "EnableTechnology", technology).Err
}

// DisableTechnology disables a technology interface.
func (m *Manager) DisableTechnology(ctx context.Context, technology string) error {
	return call(ctx, m.obj, dbusManagerInterface, "DisableTechnology", technology).Err
}

// PopAllUserProfiles removes all user profiles from the stack of managed profiles leaving only default profiles.
func (m *Manager) PopAllUserProfiles(ctx context.Context) error {
	return call(ctx, m.obj, dbusManagerInterface, "PopAllUserProfiles").Err
}
