// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

// DisplayPowerState Power state for displays.
type DisplayPowerState int

// As defined in DisplayPowerState here:
// https://cs.chromium.org/chromium/src/third_party/cros_system_api/dbus/service_constants.h
const (
	DisplayPowerAllOn                 DisplayPowerState = 0
	DisplayPowerAllOff                DisplayPowerState = 1
	DisplayPowerInternalOffExternalOn DisplayPowerState = 2
	DisplayPowerInternalOnExternalOff DisplayPowerState = 3
)

// SetDisplayPower refer to : power.SetDisplayPower
func SetDisplayPower(ctx context.Context, power DisplayPowerState) error {
	const (
		dbusName      = "org.chromium.DisplayService"
		dbusPath      = "/org/chromium/DisplayService"
		dbusInterface = "org.chromium.DisplayServiceInterface"

		setPowerMethod = "SetPower"
	)

	// restrict input range
	if power < DisplayPowerAllOn || power > DisplayPowerInternalOnExternalOff {
		return errors.Errorf("Incorrect power value: got %d, want [%d - %d]", power, DisplayPowerAllOn, DisplayPowerInternalOnExternalOff)
	}

	// set display power
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return errors.Wrapf(err, "failed to connect to %s: ", dbusName)
	}

	return obj.CallWithContext(ctx, dbusInterface+"."+setPowerMethod, 0, power).Err
}
