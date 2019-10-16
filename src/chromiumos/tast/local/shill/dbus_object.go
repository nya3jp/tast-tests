// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"
)

// DBusObject wraps D-Bus interface, object and connection needed for communication with shill.
type DBusObject struct {
	iface string
	obj   dbus.BusObject
	conn  *dbus.Conn
}

// String returns the path of the D-Bus object.
// It is so named to conform to the Stringer interface.
func (d *DBusObject) String() string {
	return string(d.obj.Path())
}

// Call calls the D-Bus method with argument against the designated D-Bus object.
func (d *DBusObject) Call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, d.iface+"."+method, 0, args...)
}
