// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECKeyboardreboot,
		Desc:         "Checks that device will reboot when EC gets the remote keyboard requests via UART",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECKeyboardreboot(ctx context.Context, s *testing.State) {

	d := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Reboot via EC normally
	s.Log("Rebooting using EC")
	if err = pxy.Servo().RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to reboot: ", err)
	}
	s.Log("Making sure the DUT is off")
	if err = d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to put DUT off while issuing reboot: ", err)
	}
	s.Log("Making sure the DUT is on")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT after reboot: ", err)
	}

	// Hard reboot via EC
	s.Log("Hard rebooting using EC")
	if err = pxy.Servo().RunECCommand(ctx, "reboot hard"); err != nil {
		s.Fatal("Failed to hard reboot: ", err)
	}
	s.Log("Making sure the DUT is off")
	if err = d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to put DUT off while issuing hard reboot: ", err)
	}
	s.Log("Making sure the DUT is on after hard reboot")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT after hard reboot: ", err)
	}

	// Reboot with ap-off via EC
	s.Log("Turning AP off using EC")
	if err = pxy.Servo().RunECCommand(ctx, "reboot ap-off"); err != nil {
		s.Fatal("Failed to put AP off: ", err)
	}
	s.Log("Making sure the DUT is off after turning AP off")
	if err = d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to put DUT off: ", err)
	}
	// "Rebooting" normally to make sure AP is on again
	s.Log("Rebooting again using EC to make AP on")
	if err = pxy.Servo().RunECCommand(ctx, "reboot"); err != nil {
		s.Fatal("Failed to reboot after making AP off: ", err)
	}
	s.Log("Making sure the DUT is on after making AP on")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to connect to DUT after making AP on: ", err)
	}

}
