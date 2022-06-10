// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Manager wraps a Hermes.Manager DBus object.
type Manager struct {
	*dbusutil.DBusObject
}

// GetHermesManager returns a Hermes manager object that can be used to list available eSIMs.
func GetHermesManager(ctx context.Context) (*Manager, error) {
	obj, err := dbusutil.NewDBusObject(ctx, hermesconst.DBusHermesService, hermesconst.DBusHermesManagerInterface, hermesconst.DBusHermesManagerPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to Hermes")
	}
	return &Manager{obj}, nil
}

// GetEUICCPaths returns the number of eUICC's on the device.
func GetEUICCPaths(ctx context.Context) ([]dbus.ObjectPath, error) {
	h, err := GetHermesManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Hermes Manager DBus object")
	}
	props, err := dbusutil.NewDBusProperties(ctx, h.DBusObject)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get Hermes manager properties")
	}
	euiccPaths, err := props.GetObjectPaths(hermesconst.ManagerPropertyAvailableEuiccs)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get available euiccs")
	}
	return euiccPaths, nil
}
