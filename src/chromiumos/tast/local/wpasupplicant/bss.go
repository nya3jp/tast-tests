// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wpasupplicant

import (
	"context"

	"github.com/godbus/dbus"
)

// BSS is the object to interact with wpa_supplicant's
// fi.w1.wpa_supplicant1.BSS interface.
type BSS struct {
	dbus *DBusObject
}

// NewBSS creates a BSS object.
func NewBSS(ctx context.Context, path dbus.ObjectPath) (*BSS, error) {
	d, err := NewDBusObject(ctx, path, "fi.w1.wpa_supplicant1.BSS")
	if err != nil {
		return nil, err
	}
	return &BSS{dbus: d}, nil
}

// BSSID returns the BSSID of this BSS.
func (b *BSS) BSSID(ctx context.Context) ([]byte, error) {
	var bssid []byte
	if err := b.dbus.Get(ctx, "BSSID", &bssid); err != nil {
		return nil, err
	}
	return bssid, nil
}
