// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/testing"
)

// DBusCloseConnection closes a DBus connection when the test completes.
func DBusCloseConnection(conn *dbus.Conn) (CleanupCallback, error) {
	return func(_ context.Context) error {
		return conn.Close()
	}, nil
}

// DBusProperty sets a DBus property to a passed value, and resets it at the end
// of the test.
func DBusProperty(ctx context.Context, obj dbus.BusObject, p string, value interface{}) (CleanupCallback, error) {
	prev, err := obj.GetProperty(p)
	if err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Setting DBus property %q from %v to %v", p, prev.Value(), value)
	if err := obj.SetProperty(p, dbus.MakeVariant(value)); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting DBus property %q to %v", p, prev.Value())
		return obj.SetProperty(p, prev)
	}, nil
}
