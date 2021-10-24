// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PowerOff,
		Desc:     "Shut down a DUT to simulate a DUT losing connectivity",
		Contacts: []string{"seewaifu@google.com"},
		VarDeps:  []string{"servo"},
	})
}

// PowerOff turns off power to simulate a DUT losing connectivity.
func PowerOff(ctx context.Context, s *testing.State) {
	// Connect to servo.
	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, s.DUT().KeyFile(), s.DUT().KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	svo := pxy.Servo()
	if err := svo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		s.Fatal("Failed to turn off power: ", err)
	}
}
