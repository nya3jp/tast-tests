// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill provides D-Bus wrappers and utilities for shill service.
package shill

import (
	"context"

	"github.com/godbus/dbus"
)

const dbusService = "org.chromium.flimflam"

// Property names defined in dbus-constants.h .
const (
	// IPConfig property names.
	IPConfigPropertyAddress     = "Address"
	IPConfigPropertyNameServers = "NameServers"
)

// call is a wrapper of dbus.BusObject.CallWithContext.
func call(ctx context.Context, obj dbus.BusObject, dbusInterface, method string, args ...interface{}) *dbus.Call {
	return obj.CallWithContext(ctx, dbusInterface+"."+method, 0, args...)
}
