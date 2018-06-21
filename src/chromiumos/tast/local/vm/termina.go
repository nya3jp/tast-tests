// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"fmt"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"

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

func LoadTerminaComponent(ctx context.Context) (string, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	testing.ContextLogf(ctx, "Mounting %q component", componentName)

	updater := bus.Object(dbusutil.ComponentUpdaterName, dbus.ObjectPath(dbusutil.ComponentUpdaterPath))
	var component_path string
	err = updater.Call(dbusutil.ComponentUpdaterInterface+".LoadComponent", 0, componentName).Store(&component_path)
	if err != nil {
		return "", fmt.Errorf("mounting %q component failed: %v", componentName, err)
	}
	testing.ContextLog(ctx, "Mounted component at path ", component_path)

	return component_path, nil
}
