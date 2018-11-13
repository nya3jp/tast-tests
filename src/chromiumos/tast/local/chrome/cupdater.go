// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"

	"github.com/godbus/dbus"

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

// NewComponentUpdater connects to ComponentUpdaterService provided by Chrome
// via D-Bus and returns ComponentUpdater object.
func NewComponentUpdater(ctx context.Context) (*ComponentUpdater, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &ComponentUpdater{conn, obj}, nil
}

// LoadComponent calls ComponentUpdaterService.LoadComponent D-Bus method.
func (c *ComponentUpdater) LoadComponent(ctx context.Context, name string, mount bool) (path string, err error) {
	cl := c.call(ctx, "LoadComponent", name, mount)
	if err = cl.Store(&path); err != nil {
		return "", err
	}
	return path, nil
}

// call is thin wrapper of CallWithContext for convenience.
func (c *ComponentUpdater) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return c.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
