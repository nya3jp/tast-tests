// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
)

// SetProperty sets a DBus property on an object. The property name is in
// "interface.property" format.
func SetProperty(ctx context.Context, obj dbus.BusObject, p string, v interface{}) error {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return errors.Errorf("invalid D-Bus property %q", p)
	}

	iface := p[:i]
	prop := p[i+1:]
	const dbusMethod = "org.freedesktop.DBus.Properties.Set"
	c := obj.CallWithContext(ctx, dbusMethod, 0, iface, prop, dbus.MakeVariant(v))
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to set D-Bus property %q on interface %q to value %v", prop, iface, v)
	}
	return nil
}

// Property gets a DBus property from an object. The property name is in
// "interface.property" format.
func Property(ctx context.Context, obj dbus.BusObject, p string) (interface{}, error) {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return dbus.MakeVariant(nil), errors.Errorf("invalid D-Bus property %q", p)
	}

	const dbusMethod = "org.freedesktop.DBus.Properties.Get"
	iface := p[:i]
	prop := p[i+1:]
	c := obj.CallWithContext(ctx, dbusMethod, 0, iface, prop)
	if c.Err != nil {
		return nil, errors.Wrapf(c.Err, "failed to get D-Bus property %q on interface %q", prop, iface)
	}

	v := dbus.Variant{}
	if err := c.Store(&v); err != nil {
		return nil, errors.Wrapf(c.Err, "failed to extract D-Bus property %q on interface %q", prop, iface)
	}
	return v.Value(), nil
}

// ManagedObjects gets all the objects managed under the passed object. Returns
// a map from interface name to a slice of ObjectPaths that have an object with
// that interface.
func ManagedObjects(ctx context.Context, obj dbus.BusObject) (map[string][]dbus.ObjectPath, error) {
	const dbusMethod = "org.freedesktop.DBus.ObjectManager.GetManagedObjects"
	c := obj.CallWithContext(ctx, dbusMethod, 0)
	if c.Err != nil {
		return nil, errors.Wrap(c.Err, "failed to get D-Bus managed objects")
	}

	v := dbus.Variant{}
	if err := c.Store(&v); err != nil {
		return nil, errors.Wrap(c.Err, "failed to extract D-Bus managed objects")
	}
	managed, ok := v.Value().(map[dbus.ObjectPath]map[string]map[string]dbus.Variant)
	if !ok {
		return nil, errors.New("cannot convert managed objects to map")
	}

	paths := map[string][]dbus.ObjectPath{}
	for objPath, ifaceToProps := range managed {
		for objIface := range ifaceToProps {
			paths[objIface] = append(paths[objIface], objPath)
		}
	}
	return paths, nil
}
