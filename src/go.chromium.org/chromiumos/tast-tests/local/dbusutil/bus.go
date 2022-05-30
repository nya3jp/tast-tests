// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import "github.com/godbus/dbus/v5"

const (
	busName      = "org.freedesktop.DBus"                   // system bus service name
	busPath      = dbus.ObjectPath("/org/freedesktop/DBus") // system bus service path
	busInterface = "org.freedesktop.DBus"                   // system bus interface
)
