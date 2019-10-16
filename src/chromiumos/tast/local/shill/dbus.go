// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import (
	"context"

	"github.com/godbus/dbus"
)

// DBus wraps D-Bus interface, object and connection needed for communication with shill.
type DBus struct {
	Interface string
	Object    dbus.BusObject
	Conn      *dbus.Conn
}

// Call calls the D-Bus method with argument against the designated D-Bus object.
func (d *DBus) Call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return call(ctx, d.Object, d.Interface, method, args...)
}
