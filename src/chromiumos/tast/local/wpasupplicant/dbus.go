// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

// TODO(b:161486698): This objects are stolen and extended from shill package. Probably worth to
// extract them into common component.

const dbusGetPropsMethod = "org.freedesktop.DBus.Properties.Get"

// NewDBusObject creates a DBusObject to wpa_supplicant.
func NewDBusObject(ctx context.Context, path dbus.ObjectPath, iface string) (*DBusObject, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusBaseInterface, path)
	if err != nil {
		return nil, err
	}
	return &DBusObject{
		iface: iface,
		conn:  conn,
		obj:   obj,
	}, nil
}

// DBusObject wraps D-Bus interface, object and connection needed for communication with wpa_supplicant.
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

// Get calls org.freedesktop.DBus.Properties.Get and store the result into val.
func (d *DBusObject) Get(ctx context.Context, propName string, val interface{}) error {
	return d.obj.CallWithContext(ctx, dbusGetPropsMethod, 0, d.iface, propName).Store(val)
}

// CreateWatcher returns a SignalWatcher to observe the specified signal.
func (d *DBusObject) CreateWatcher(ctx context.Context, signalName string) (*dbusutil.SignalWatcher, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      d.obj.Path(),
		Interface: d.iface,
		Member:    signalName,
	}
	watcher, err := dbusutil.NewSignalWatcher(ctx, d.conn, spec)
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
