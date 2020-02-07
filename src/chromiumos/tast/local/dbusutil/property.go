// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dbusutil

import (
	"context"
	"strings"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
)

// SetProperty sets a DBus property on an object. The property name is in
// "interface.property" format.
func SetProperty(ctx context.Context, obj dbus.BusObject, p string, v interface{}) error {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return errors.Errorf("invalid DBus property %q", p)
	}

	iface := p[:i]
	prop := p[i+1:]
	const dbusMethod = "org.freedesktop.DBus.Properties.Set"
	c := obj.CallWithContext(ctx, dbusMethod, 0, iface, prop, dbus.MakeVariant(v))
	if c.Err != nil {
		return errors.Wrapf(c.Err, "failed to set DBUS property %q on interface %q to value %v", prop, iface, v)
	}
	return nil
}

// Property gets a DBus property from an object. The property name is in
// "interface.property" format.
func Property(ctx context.Context, obj dbus.BusObject, p string) (interface{}, error) {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return dbus.MakeVariant(nil), errors.Errorf("invalid DBus property %q", p)
	}

	const dbusMethod = "org.freedesktop.DBus.Properties.Get"
	iface := p[:i]
	prop := p[i+1:]
	c := obj.CallWithContext(ctx, dbusMethod, 0, iface, prop)
	if c.Err != nil {
		return nil, errors.Wrapf(c.Err, "failed to get DBUS property %q on interface %q", prop, iface)
	}

	v := dbus.Variant{}
	if err := c.Store(&v); err != nil {
		return nil, errors.Wrapf(c.Err, "failed to extract DBUS property %q on interface %q", prop, iface)
	}
	return v.Value(), nil
}
