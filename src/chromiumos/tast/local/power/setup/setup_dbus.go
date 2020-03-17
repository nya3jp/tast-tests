// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

// DBusProperty sets a DBus property to a passed value, and resets it at the end
// of the test.
func DBusProperty(ctx context.Context, obj dbus.BusObject, p string, value interface{}) (CleanupCallback, error) {
	prev, err := dbusutil.Property(ctx, obj, p)
	if err != nil {
		return nil, err
	}

	if err := dbusutil.SetProperty(ctx, obj, p, value); err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		return dbusutil.SetProperty(ctx, obj, p, prev)
	}, nil
}
