// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"
	"fmt"

	"github.com/godbus/dbus"
)

// MustGetSystemBus returns the shared connection to the D-Bus system bus.
// It panics if an error is returned (which is not expected, as the system bus is required by Chrome OS).
// Close must not be called on the returned connection.
func MustGetSystemBus(ctx context.Context) *dbus.Conn {
	conn, err := dbus.SystemBus()
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to D-Bus system bus: %v", err))
	}
	return conn
}
