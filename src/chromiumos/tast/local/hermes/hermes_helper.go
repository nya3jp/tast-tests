// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hermes

import (
	"context"

	"chromiumos/tast/common/hermesconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// Modem wraps a Modemmanager.Modem D-Bus object.
type Hermes struct {
	*dbusutil.DBusObject
}

type Euicc struct {
	*dbusutil.DBusObject
}

type Profile struct {
	*dbusutil.DBusObject
}

func GetHermes(ctx context.Context) (*Hermes, error) {
		obj, err := dbusutil.NewDBusObject(ctx, DBusHermesService, DBusHermesManagerInterface, DBusHermesManagerPath)
		if err != nil {
			return nil,errors.Wrap(err, "Unable to connect to Hermes")
		}
		return &Hermes{obj}, nil
}

// GetEuiccs creates a new PropertyHolder instance for each euicc object.
func GetEuicc(ctx context.Context, euiccNum int) (*Euicc, error) {
	h, err := GetHermes(ctx);
		if err != nil {
			return nil,errors.Wrap(err, "Could not get Hermes Manager dbus object")
		}

	props, err := dbusutil.NewDBusProperties(ctx, h.DBusObject)
		if err != nil {
			return nil,errors.Wrap(err, "Unable to get Hermes manager properties")
		}
	euiccPaths,err := props.GetObjectPaths(hermesconst.HermesManagerPropertyAvailableEuiccs)
		if err != nil {
			return nil,errors.Wrap(err, "Unable to get available euiccs")
		}
	// TODO: check euiccNum bounds
	obj, err := dbusutil.NewDBusObject(ctx, DBusHermesService, DBusHermesEuiccInterface, euiccPaths[euiccNum])
		if err != nil {
			return nil,errors.Wrap(err, "Unable to get Euicc")
		}
	return &Euicc{obj}, nil
}
