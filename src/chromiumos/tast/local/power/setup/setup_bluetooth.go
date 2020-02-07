// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func dbusPropertyGet(ctx context.Context, obj dbus.BusObject, iface, prop string, outValue interface{}) error {
	const dbusMethod = "org.freedesktop.DBus.Properties.Get"
	c := obj.CallWithContext(ctx, dbusMethod, 0, iface, prop)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to get DBUS property %q on interface %q", prop, iface)
	}
	if err := c.Store(outValue); err != nil {
		return errors.Wrapf(err, "failed to read DBUS property value %q on interface %q", prop, iface)
	}
	return nil
}

func dbusPropertySet(ctx context.Context, obj dbus.BusObject, iface, prop string, value dbus.Variant) error {
	const dbusMethod = "org.freedesktop.DBus.Properties.Set"
	c := obj.CallWithContext(ctx, dbusMethod, 0, iface, prop, value)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to set DBUS property %q on interface %q", prop, iface)
	}
	return nil
}

// DisableBluetooth disables the bluetooth adapter on the DUT.
func DisableBluetooth(ctx context.Context) (CleanupCallback, error) {
	const (
		dbusName      = "org.bluez"
		dbusPath      = "/org/bluez/hci0"
		dbusInterface = "org.bluez.Adapter1"
		dbusProperty  = "Powered"
	)

	testing.ContextLog(ctx, "Disabling Bluetooth")
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create DBUS connection to Bluetooth adapter")
	}

	var enabled bool
	if err := dbusPropertyGet(ctx, obj, dbusInterface, dbusProperty, &enabled); err != nil {
		return nil, err
	}
	if !enabled {
		testing.ContextLog(ctx, "Not disabling Bluetoot, already disabled")
		return nil, nil
	}

	disabled := !enabled
	if err := dbusPropertySet(ctx, obj, dbusInterface, dbusProperty, dbus.MakeVariant(&disabled)); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		testing.ContextLog(ctx, "Re-enabling Bluetooth")
		if err := dbusPropertySet(ctx, obj, dbusInterface, dbusProperty, dbus.MakeVariant(&enabled)); err != nil {
			return err
		}
		if err := conn.Close(); err != nil {
			return errors.Wrap(err, "failed to disconnect DBUS connection to Bluetooth adapter")
		}
		return nil
	}, nil
}
