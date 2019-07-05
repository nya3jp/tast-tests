// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package display

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.DisplayService"
	dbusPath      = "/org/chromium/DisplayService"
	dbusInterface = "org.chromium.DisplayServiceInterface"
)

// DisplayService is used to interact with the display over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/system_api/dbus/service_constants.h
type DisplayService struct { // NOLINT
	conn *dbus.Conn
	obj  dbus.BusObject
}

// PowerState is a status code for the display related D-Bus methods.
// Values are from src/platform2/system_api/dbus/service_constants.h
type PowerState int32

const (
	// PowerAllOn means all displays on.
	PowerAllOn PowerState = 0

	// PowerAllOff means all displays off.
	PowerAllOff PowerState = 1

	// PowerInternalOffExternalOn means internal display off and external displays on.
	PowerInternalOffExternalOn PowerState = 2

	// PowerInternalOnExternalOff means internal display on and external displays off.
	PowerInternalOnExternalOff PowerState = 3
)

// NewDisplayService connects to display service via D-Bus and returns a DisplayService object.
func NewDisplayService(ctx context.Context) (*DisplayService, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &DisplayService{conn, obj}, nil
}

// SetPower sets power state of the display.
func (m *DisplayService) SetPower(ctx context.Context, state PowerState) *dbus.Call {
	return m.obj.CallWithContext(ctx, dbusInterface+".SetPower", 0, state)
}

// TurnOnDisplay turns on the display.
func (m *DisplayService) TurnOnDisplay(ctx context.Context) *dbus.Call {
	return m.SetPower(ctx, PowerAllOn)
}
