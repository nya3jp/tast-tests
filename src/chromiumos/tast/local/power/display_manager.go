// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"github.com/godbus/dbus/v5"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusDisplayName      = "org.chromium.DisplayService"
	dbusDisplayPath      = "/org/chromium/DisplayService"
	dbusDisplayInterface = "org.chromium.DisplayServiceInterface"
)

// DisplayManager is used to interact with the display service over D-Bus.
type DisplayManager struct { // NOLINT
	conn *dbus.Conn
	obj  dbus.BusObject
}

// DisplayPowerStatus is setting to change the power status of the display.
type DisplayPowerStatus int

// These consts are from /src/platform2/system_api/dbus/service_constants.h
const (
	DisplayPowerAllOn DisplayPowerStatus = iota
	DisplayPowerAllOff
	DisplayPowerInternalOffExternalOn
	DisplayPowerInternalOnExternalOff
)

// NewDisplayManager connects to display_service via D-Bus and returns a DisplayManager object.
func NewDisplayManager(ctx context.Context) (*DisplayManager, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusDisplayName, dbusDisplayPath)
	if err != nil {
		return nil, err
	}
	return &DisplayManager{conn, obj}, nil
}

// SetDisplayPower sets the display values by sending SetPower to DisplayService.
func SetDisplayPower(ctx context.Context, powerStatus DisplayPowerStatus) error {
	displayd, err := NewDisplayManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a DisplayManager object")
	}

	if err := displayd.obj.CallWithContext(ctx, dbusDisplayInterface+".SetPower", 1, powerStatus).Err; err != nil {
		return errors.Wrap(err, "faild to call SetPower D-Bus method")
	}
	return nil
}
