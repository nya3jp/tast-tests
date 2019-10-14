// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shill

import "github.com/godbus/dbus"

// DBusObject wraps D-Bus interface, object and connection needed for communication with shill.
type DBusObject struct {
	Interface string
	Object    dbus.BusObject
	Conn      *dbus.Conn
}
