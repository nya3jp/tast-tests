// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Manager wraps a Hermes.Manager DBus object.
type Manager struct {
	*dbusutil.DBusObject
}

// GetHermesManager returns a Hermes manager object that can be used to list available eSIMs.
func GetHermesManager(ctx context.Context) (*Manager, error) {
	obj, err := dbusutil.NewDBusObject(ctx, DBusHermesService, DBusHermesManagerInterface, DBusHermesManagerPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to Hermes")
	}
	return &Manager{obj}, nil
}
