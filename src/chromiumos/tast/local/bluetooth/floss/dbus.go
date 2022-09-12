// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package floss provides a Floss implementation of the Bluetooth interface.
package floss

import (
	"context"

	"chromiumos/tast/local/dbusutil"
)

const (
	managerService   = "org.chromium.bluetooth.Manager"
	managerInterface = "org.chromium.bluetooth.Manager"
	managerObject    = "/org/chromium/bluetooth/Manager"
)

func newManagerDBusObject(ctx context.Context) (*dbusutil.DBusObject, error) {
	return dbusutil.NewDBusObject(ctx, managerService, managerInterface, managerObject)
}
