// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wpasupplicant provides utilities to interact with wpa_supplicant
// via dbus.
package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

const (
	dbusBasePath           = "/fi/w1/wpa_supplicant1"
	dbusBaseInterface      = "fi.w1.wpa_supplicant1"
	dbusBaseGetIfaceMethod = "GetInterface"
)

// The following are some of the expected values of the property DisconnectReason.
const (
	// DisconnReasonPreviousAuthenticationInvalid previous authentication no longer valid.
	DisconnReasonPreviousAuthenticationInvalid = 2
	// DisconnReasonDeauthSTALeaving deauthenticated because sending STA is leaving (or has left) IBSS or ESS.
	DisconnReasonDeauthSTALeaving = 3
	// DisconnReasonLGDeauthSTALeaving (locally generated).
	DisconnReasonLGDeauthSTALeaving = -3
	// DisconnReasonLGDisassociatedInactivity (locally generated) disassociated due to inactivity.
	DisconnReasonLGDisassociatedInactivity = -4
)

// Supplicant is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1 interface.
type Supplicant struct {
	dbus *dbusutil.DBusObject
}

// NewSupplicant creates a Supplicant object.
func NewSupplicant(ctx context.Context) (*Supplicant, error) {
	d, err := dbusutil.NewDBusObject(ctx, dbusBaseInterface, dbusBaseInterface, dbusBasePath)
	if err != nil {
		return nil, err
	}
	return &Supplicant{dbus: d}, nil
}

// GetInterface calls fi.w1.wpa_supplicant1.GetInterface to get the object path of the
// interface with name and return the Interface object with the object path.
func (s *Supplicant) GetInterface(ctx context.Context, name string) (*Interface, error) {
	var path dbus.ObjectPath
	if err := s.dbus.Call(ctx, dbusBaseGetIfaceMethod, name).Store(&path); err != nil {
		return nil, err
	}
	return NewInterface(ctx, path)
}
