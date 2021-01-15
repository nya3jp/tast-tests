// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

const (
	dbusBSSInterface = "fi.w1.wpa_supplicant1.BSS"
	dbusBSSPropBSSID = "BSSID"
	dbusBSSPropSSID  = "SSID"
)

// BSS is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1.BSS interface.
type BSS struct {
	dbus *dbusutil.DBusObject
}

// NewBSS creates a BSS object.
func NewBSS(ctx context.Context, path dbus.ObjectPath) (*BSS, error) {
	d, err := dbusutil.NewDBusObject(ctx, dbusBaseInterface, dbusBSSInterface, path)
	if err != nil {
		return nil, err
	}
	return &BSS{dbus: d}, nil
}

// BSSID returns the BSSID of this BSS.
func (b *BSS) BSSID(ctx context.Context) ([]byte, error) {
	var bssid []byte
	if err := b.dbus.Get(ctx, dbusBSSPropBSSID, &bssid); err != nil {
		return nil, err
	}
	return bssid, nil
}

// SSID returns the SSID of this BSS.
func (b *BSS) SSID(ctx context.Context) ([]byte, error) {
	var ssid []byte
	if err := b.dbus.Get(ctx, dbusBSSPropSSID, &ssid); err != nil {
		return nil, err
	}
	return ssid, nil
}
