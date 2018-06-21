// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"

	"chromiumos/tast/local/dbusutil"

	"github.com/godbus/dbus"
)

const (
	componentName = "cros-termina" // The name of the Chrome component for the VM kernel and rootfs.
)

func getDBusObject() (obj dbus.BusObject, err error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	return bus.Object(dbusutil.ConciergeName, dbus.ObjectPath(dbusutil.ConciergePath)), nil
}

// LoadTerminaComponent loads the termina component that contains the VM kernel
// and rootfs. The path of the loaded component is returned. This is needed
// before running VMs.
func LoadTerminaComponent(ctx context.Context) (string, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	updater := bus.Object(dbusutil.ComponentUpdaterName, dbus.ObjectPath(dbusutil.ComponentUpdaterPath))
	var componentPath string
	err = updater.Call(dbusutil.ComponentUpdaterInterface+".LoadComponent", 0, componentName).Store(&componentPath)
	if err != nil {
		return "", fmt.Errorf("mounting %q component failed: %v", componentName, err)
	}

	return componentPath, nil
}
