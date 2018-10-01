// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package debugd interacts with debugd D-Bus service.
package debugd

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	dbusName      = "org.chromium.debugd"
	dbusPath      = "/org/chromium/debugd"
	dbusInterface = "org.chromium.debugd"
)

// Debugd is used to interact with the debugd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/debugd/dbus_bindings/org.chromium.debugd.xml
type Debugd struct {
	obj dbus.BusObject
}

func New(ctx context.Context) (*Debugd, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("Failed connection to system bus: %v", err)
	}

	testing.ContextLogf(ctx, "Waiting for %s D-Bus service", dbusName)
	if err := dbusutil.WaitForService(ctx, conn, dbusName); err != nil {
		return nil, fmt.Errorf("Failed waiting for Debugd service: %v", err)
	}

	obj := conn.Object(dbusName, dbusPath)
	return &Debugd{obj}, nil
}

// CupsAddAutoConfiguredPrinter calls debugd.CupsAddAutoCOnfiguredPrinter D-Bus method.
func (d *Debugd) CupsAddAutoConfiguredPrinter(ctx context.Context, name, uri string) (int32, error) {
	c := d.call(ctx, "CupsAddAutoConfiguredPrinter", name, uri)
	var status int32
	if err := c.Store(&status); err != nil {
		return 0, err
	}
	return status, nil
}

// CupsAddManuallyConfiguredPrinter calls debugd.CupsAddManuallyConfiguredPrinter D-Bus method.
func (d *Debugd) CupsAddManuallyConfiguredPrinter(ctx context.Context, name, uri string, ppdContents []byte) (int32, error) {
	c := d.call(ctx, "CupsAddManuallyConfiguredPrinter", name, uri, ppdContents)
	var status int32
	if err := c.Store(&status); err != nil {
		return 0, err
	}
	return status, nil
}

// call is thin wrapper of CallWithContext for convenience.
func (d *Debugd) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
