// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
)

const (
	dbusWpasupplicantObjIface = "fi.w1.wpa_supplicant1.Interface"
	dbusWpasupplicantObjBSSs  = "BSSs"
)

// Interface is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1.Interface interface.
type Interface struct {
	dbus *DBusObject
}

// NewInterface creates an Interface object.
func NewInterface(ctx context.Context, path dbus.ObjectPath) (*Interface, error) {
	d, err := NewDBusObject(ctx, path, dbusWpasupplicantObjIface)
	if err != nil {
		return nil, err
	}
	return &Interface{dbus: d}, nil
}

// BSSs returns the BSSs property of the interface.
func (iface *Interface) BSSs(ctx context.Context) ([]*BSS, error) {
	var bssPaths []dbus.ObjectPath
	if err := iface.dbus.Get(ctx, dbusWpasupplicantObjBSSs, &bssPaths); err != nil {
		return nil, err
	}
	var ret []*BSS
	for _, path := range bssPaths {
		bss, err := NewBSS(ctx, path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to NewBSS(%s)", path)
		}
		ret = append(ret, bss)
	}
	return ret, nil
}
