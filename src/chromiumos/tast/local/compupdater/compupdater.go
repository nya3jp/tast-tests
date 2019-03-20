// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package compupdater provides D-Bus proxy interface to communicate with
// ComponentUpdaterService provided by Chrome.
package compupdater

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.ComponentUpdaterService"
	dbusPath      = "/org/chromium/ComponentUpdaterService"
	dbusInterface = "org.chromium.ComponentUpdaterService"
)

// ComponentUpdater is used to interact with the ComponentUpdater provided
// by Chrome over D-Bus.
// For detailed spec of each D-Bus method, please find
// chrome/browser/chromeos/dbus/component_updater_service_provider.h in Chrome
// repository.
type ComponentUpdater struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// MountMode is the mode for LoadComponent about whether the component should
// be mounted.
type MountMode bool

const (
	// NoMount indicates "do not mount on LoadComponent".
	NoMount MountMode = false
	// Mount indicates "mount on LoadComponent".
	Mount MountMode = true
)

// New connects to ComponentUpdaterService provided by Chrome
// via D-Bus and returns ComponentUpdater object.
func New(ctx context.Context) (*ComponentUpdater, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &ComponentUpdater{conn, obj}, nil
}

// LoadComponent calls ComponentUpdaterService.LoadComponent D-Bus method.
func (c *ComponentUpdater) LoadComponent(ctx context.Context, name string, mount MountMode) (path string, err error) {
	cl := c.call(ctx, "LoadComponent", name, mount)
	if err = cl.Store(&path); err != nil {
		return "", err
	}
	if mount == Mount && path == "" {
		return path, errors.New("component installation failed")
	}
	return path, nil
}

// UnloadComponent calls ComponentUpdaterService.UnloadComponent D-Bus method.
func (c *ComponentUpdater) UnloadComponent(ctx context.Context, name string) error {
	return c.call(ctx, "UnloadComponent", name).Err
}

// call is thin wrapper of CallWithContext for convenience.
func (c *ComponentUpdater) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
