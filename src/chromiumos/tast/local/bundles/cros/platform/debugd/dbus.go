// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.debugd"
	dbusPath      = "/org/chromium/debugd"
	dbusInterface = "org.chromium.debugd"
)

// debugd is used to interact with the debugd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/debugd/dbus_bindings/org.chromium.debugd.xml.
type debugd struct {
	conn *dbus.Conn
	obj  dbus.BusObject
}

// newDebugd connects to debugd via D-Bus and returns a debugd object.
func newDebugd(ctx context.Context) (*debugd, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &debugd{conn, obj}, nil
}

// call is a thing wrapper of CallWithContext for convenience.
func (d *debugd) call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}

// setSchedulerConfiguration calls debugd.SetSchedulerConfiguration D-Bus method.
func (d *debugd) setSchedulerConfiguration(ctx context.Context, param string) (bool, error) {
	var result bool
	if err := d.call(ctx, "SetSchedulerConfiguration", param).Store(&result); err != nil {
		return false, err
	}
	return result, nil
}
