// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servoutil

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/testing"
)

// TurnOffServoKeyboard turns off servo keyboard.
func TurnOffServoKeyboard(ctx context.Context, s *testing.State) {
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	if err := pxy.Servo().SetOnOff(ctx, servo.USBKeyboard, servo.Off); err != nil {
		s.Fatal("Failed to turn of servo: ", err)
	}
}
