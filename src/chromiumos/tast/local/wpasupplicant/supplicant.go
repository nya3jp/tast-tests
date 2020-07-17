// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpasupplicant provides utilities to interact with wpa_supplicant
// via dbus.
package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"
)

const (
	dbusWpasupplicantPath      = "/fi/w1/wpa_supplicant1"
	dbusWpasupplicantInterface = "fi.w1.wpa_supplicant1"
	dbusGetIfaceMethod         = "GetInterface"
)

// Supplicant is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1 interface.
type Supplicant struct {
	dbus *DBusObject
}

// NewSupplicant creates a Supplicant object.
func NewSupplicant(ctx context.Context) (*Supplicant, error) {
	d, err := NewDBusObject(ctx, dbusWpasupplicantPath, dbusWpasupplicantInterface)
	if err != nil {
		return nil, err
	}
	return &Supplicant{dbus: d}, nil
}

// GetInterface calls fi.w1.wpa_supplicant1.GetInterface to get the object path of the
// interface with name and return the Interface object with the object path.
func (s *Supplicant) GetInterface(ctx context.Context, name string) (*Interface, error) {
	var path dbus.ObjectPath
	if err := s.dbus.Call(ctx, dbusGetIfaceMethod, name).Store(&path); err != nil {
		return nil, err
	}
	return NewInterface(ctx, path)
}
