// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"

	"github.com/godbus/dbus"
)

// DBusObject wraps a D-Bus interface, object and connection.
type DBusObject struct {
	iface string
	obj   dbus.BusObject
	conn  *dbus.Conn
}

const dbusGetPropsMethod = "org.freedesktop.DBus.Properties.Get"
const dbusGetAllPropsMethod = "org.freedesktop.DBus.Properties.GetAll"

// NewDBusObject creates a DBusObject.
func NewDBusObject(ctx context.Context, service, iface string, path dbus.ObjectPath) (*DBusObject, error) {
	conn, obj, err := ConnectNoTiming(ctx, service, path)
	if err != nil {
		return nil, err
	}
	return &DBusObject{
		iface: iface,
		conn:  conn,
		obj:   obj,
	}, nil
}

// ObjectPath returns the path of the D-Bus object.
func (d *DBusObject) ObjectPath() dbus.ObjectPath {
	return d.obj.Path()
}

// String returns the path of the D-Bus object as a string.
// It is so named to conform to the Stringer interface.
func (d *DBusObject) String() string {
	return string(d.obj.Path())
}

// Call calls the D-Bus method with argument against the designated D-Bus object.
func (d *DBusObject) Call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, d.iface+"."+method, 0, args...)
}

// Get calls org.freedesktop.DBus.Properties.Get and stores the result into val.
func (d *DBusObject) Get(ctx context.Context, propName string, val interface{}) error {
	return d.obj.CallWithContext(ctx, dbusGetPropsMethod, 0, d.iface, propName).Store(val)
}

// GetAll calls org.freedesktop.DBus.Properties.GetAll and stores the result into val.
func (d *DBusObject) GetAll(ctx context.Context) (map[string]interface{}, error) {
	var val map[string]interface{}
	if err := d.obj.CallWithContext(ctx, dbusGetAllPropsMethod, 0, d.iface).Store(val); err != nil {
		return nil, err
	}
	return val, nil
}

// CreateWatcher returns a SignalWatcher to observe the specified signals.
func (d *DBusObject) CreateWatcher(ctx context.Context, signalNames ...string) (*SignalWatcher, error) {
	specs := make([]MatchSpec, len(signalNames))
	for i, sigName := range signalNames {
		specs[i] = MatchSpec{
			Type:      "signal",
			Path:      d.obj.Path(),
			Interface: d.iface,
			Member:    sigName,
		}
	}
	watcher, err := NewSignalWatcher(ctx, d.conn, specs...)
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
