// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"strings"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
)

func objSetProperty(obj dbus.BusObject, p string, v interface{}) error {
	idx := strings.LastIndex(p, ".")
	if idx == -1 || idx+1 == len(p) {
		return errors.New("dbus: invalid property " + p)
	}

	iface := p[:idx]
	prop := p[idx+1:]
	const dbusMethod = "org.freedesktop.DBus.Properties.Set"
	c := obj.Call(dbusMethod, 0, iface, prop, v)
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to set DBUS property %q on interface %q", prop, iface)
	}
	return nil
}

// DBusCloseConnection closes a DBus connection when the test completes.
func DBusCloseConnection(conn *dbus.Conn) (CleanupCallback, error) {
	return func(_ context.Context) error {
		return conn.Close()
	}, nil
}

// DBusProperty sets a DBus property to a passed value, and resets it at the end
// of the test.
func DBusProperty(obj dbus.BusObject, p string, value interface{}) (CleanupCallback, error) {
	prev, err := obj.GetProperty(p)
	if err != nil {
		return nil, err
	}

	if err := objSetProperty(obj, p, dbus.MakeVariant(value)); err != nil {
		return nil, err
	}

	return func(_ context.Context) error {
		return objSetProperty(obj, p, prev)
	}, nil
}
