// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bluetoothutil provides common functions used by bluetooth test.
package bluetoothutil

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/testing"
)

// TurnOfServoKeyboardIfOn turns off servo keyboard if on.
func TurnOfServoKeyboardIfOn(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	isOn, err := pxy.Servo().GetOnOff(ctx, servo.USBKeyboard)

	if err != nil {
		s.Fatal("Failed to turn of servo: ", err)
	}

	if isOn {
		if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
			s.Fatal("Failed to turn of servo: ", err)
		}
	}
}
