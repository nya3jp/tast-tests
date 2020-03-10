// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// DisableBluetooth disables the bluetooth adapter on the DUT.
func DisableBluetooth(ctx context.Context) (CleanupCallback, error) {
	return Nested(ctx, "disable Bluetooth", func(s *Setup) error {
		const (
			name     = "org.bluez"
			path     = "/org/bluez/hci0"
			property = "org.bluez.Adapter1.Powered"
		)
		conn, obj, err := dbusutil.Connect(ctx, name, path)
		if err != nil {
			return errors.Wrap(err, "failed to create DBUS connection to Bluetooth adapter")
		}
		s.Add(DBusCloseConnection(conn))
		s.Add(DBusProperty(ctx, obj, property, false))
		return nil
	})
}
