// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"

	"github.com/godbus/dbus/v5"
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

// Obj returns the dbus.BusObject for this object.
func (d *DBusObject) Obj() dbus.BusObject {
	return d.obj
}

// Iface returns the D-Bus interface for this type of object.
func (d *DBusObject) Iface() string {
	return d.iface
}

// Conn returns the D-Bus connection to service.
func (d *DBusObject) Conn() *dbus.Conn {
	return d.conn
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

// IfacePath builds a D-Bus path starting at this object's interface path.
func (d *DBusObject) IfacePath(subPath string) string {
	return BuildIfacePath(d.iface, subPath)
}

// Call calls the D-Bus method with argument against the designated D-Bus object.
func (d *DBusObject) Call(ctx context.Context, method string, args ...interface{}) *dbus.Call {
	return d.obj.CallWithContext(ctx, d.IfacePath(method), 0, args...)
}

// Property calls org.freedesktop.DBus.Properties.Get and stores the result into
// val.
func (d *DBusObject) Property(ctx context.Context, propName string, val interface{}) error {
	return d.obj.CallWithContext(ctx, dbusGetPropsMethod, 0, d.iface, propName).Store(val)
}

// PropertyBool calls Property for a bool value.
func (d *DBusObject) PropertyBool(ctx context.Context, propName string) (bool, error) {
	var value bool
	if err := d.Property(ctx, propName, &value); err != nil {
		return false, err
	}
	return value, nil
}

// PropertyString calls Property for a string value.
func (d *DBusObject) PropertyString(ctx context.Context, propName string) (string, error) {
	var value string
	if err := d.Property(ctx, propName, &value); err != nil {
		return "", err
	}
	return value, nil
}

// PropertyStrings calls Property for a []string value.
func (d *DBusObject) PropertyStrings(ctx context.Context, propName string) ([]string, error) {
	var value []string
	if err := d.Property(ctx, propName, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// AllProperties calls org.freedesktop.DBus.Properties.GetAll and stores the
// result into val.
func (d *DBusObject) AllProperties(ctx context.Context) (map[string]interface{}, error) {
	val := make(map[string]interface{})
	if err := d.obj.CallWithContext(ctx, dbusGetAllPropsMethod, 0, d.iface).Store(val); err != nil {
		return nil, err
	}
	return val, nil
}

// SetProperty is a shortcut for calling SetProperty for this object.
// The property name is prepended with the iface path.
func (d *DBusObject) SetProperty(ctx context.Context, name string, value interface{}) error {
	return SetProperty(ctx, d.obj, d.IfacePath(name), value)
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

// BuildIfacePath builds a full D-Bus path starting at the given baseIface path.
func BuildIfacePath(baseIface, subPath string) string {
	return baseIface + "." + subPath
}
