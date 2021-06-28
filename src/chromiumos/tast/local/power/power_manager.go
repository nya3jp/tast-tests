// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	pmpb "chromiumos/system_api/power_manager_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
)

const (
	dbusName      = "org.chromium.PowerManager"
	dbusPath      = "/org/chromium/PowerManager"
	dbusInterface = "org.chromium.PowerManager"
)

// PowerManager is used to interact with the powerd process over D-Bus.
// For detailed spec of each D-Bus method, please find
// src/platform2/power_manager/dbus_bindings/org.chromium.PowerManager.xml
type PowerManager struct { // NOLINT
	conn *dbus.Conn
	obj  dbus.BusObject
}

// UserActivityType is a status code for the PowerManager related D-Bus methods.
type UserActivityType int32

// NewPowerManager connects to power_manager via D-Bus and returns a PowerManager object.
func NewPowerManager(ctx context.Context) (*PowerManager, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &PowerManager{conn, obj}, nil
}

// GetSwitchStates calls PowerManager.GetSwitchStates D-Bus method.
func (m *PowerManager) GetSwitchStates(ctx context.Context) (*pmpb.SwitchStates, error) {
	ret := &pmpb.SwitchStates{}
	err := dbusutil.CallProtoMethod(ctx, m.obj, dbusInterface+".GetSwitchStates", nil, ret)
	return ret, err
}

// HandleWakeNotification calls PowerManager.HandleWakeNotification D-Bus method.
func (m *PowerManager) HandleWakeNotification(ctx context.Context) error {
	return dbusutil.CallProtoMethod(ctx, m.obj, dbusInterface+".HandleWakeNotification", nil, nil)
}

// TurnOnDisplay turns on a display by sending a HandleWakeNotification to PowerManager
// to light up the display.
func TurnOnDisplay(ctx context.Context) error {
	// Emitting wake notification to powerd should finish quickly -- so setting
	// 10 seconds of timeout which should be long enough.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := upstart.EnsureJobRunning(ctx, "powerd"); err != nil {
		return errors.Wrap(err, "failed to ensure powerd running")
	}

	powerd, err := NewPowerManager(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a PowerManager object")
	}
	if err := powerd.HandleWakeNotification(ctx); err != nil {
		return errors.Wrap(err, "failed to call HandleWakeNotification D-Bus method")
	}
	return nil
}

// UpdateBrightness updates the screen brightness to the specified value and returns a callback to revert it back to the original.
func (m *PowerManager) UpdateBrightness(ctx context.Context, value float64) (func(context.Context) error, error) {
	call := m.obj.CallWithContext(ctx, dbusInterface+".GetScreenBrightnessPercent", 0)
	if call.Err != nil {
		return nil, errors.Wrap(call.Err, "failed to call dbus method GetScreenBrightnessPercent")
	}

	var prev float64
	if err := call.Store(&prev); err != nil {
		return nil, errors.Wrap(err, "failed to store dbus method call response")
	}

	if err := dbusutil.CallProtoMethod(ctx, m.obj, dbusInterface+".SetScreenBrightness",
		&pmpb.SetBacklightBrightnessRequest{
			Percent: &value,
		}, nil); err != nil {
		return nil, errors.Wrapf(err, "unable to alter screen brightness from %f to %f", prev, value)
	}

	return func(ctx context.Context) error {
		if err := dbusutil.CallProtoMethod(ctx, m.obj, dbusInterface+".SetScreenBrightness",
			&pmpb.SetBacklightBrightnessRequest{
				Percent: &prev,
			}, nil); err != nil {
			return errors.Wrapf(err, "failed to reset screen brightness from %f to %f", value, prev)
		}
		return nil
	}, nil
}
