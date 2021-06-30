// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECHibernate,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECHibernate(ctx context.Context, s *testing.State) {

	d := s.DUT()

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Tell servo to stop supplying power.
	s.Log("Disable charge-through")
	if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Error disabling charge-through: ", err)
	}

	// Wait for a short delay between cutting power supply and telling EC to hibernate.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Verify that the power is off.
	s.Log("Check that DUT's power source has been cut off")
	ok, err := pxy.Servo().GetChargerAttached(ctx)
	if err != nil {
		s.Fatal("Failed to check whether power is off: ", err)
	}
	if ok {
		s.Fatal("Power was still on after disabling servo charge-through")
	}
	s.Logf("Charger attached: %t", ok)

	// Tell EC to hibernate.
	s.Log("Put DUT in hibernation mode")
	if err = pxy.Servo().RunECCommand(ctx, "hibernate"); err != nil {
		s.Fatal("Failed to hibernate: ", err)
	}

	// Wait for a short delay after putting DUT in hibernation.
	if err := testing.Sleep(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Verify that EC is non-responsive by querying an EC command.
	s.Log("Verify EC is non-responsive")
	waitECCtx, cancelEC := context.WithTimeout(ctx, 10*time.Second)
	defer cancelEC()

	// Expect no return for the query, and receive error of type FaultError.
	_, errEC := pxy.Servo().RunECCommandGetOutput(waitECCtx, "version", []string{`.`})
	if errEC == nil {
		s.Fatal("EC was still responsive after putting DUT in hibernation")
	}
	var errSend xmlrpc.FaultError
	if !errors.As(errEC, &errSend) {
		s.Fatal("EC was still responsive after putting DUT in hibernation: ", errEC)
	}
	s.Log("EC was non-responsive")

	// Wake EC up by telling servo to re-supply power.
	s.Log("Enable charge-through")
	if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Error enabling charge-through: ", err)
	}

	// Wait for DUT to reboot and reconnect.
	s.Log("Wait for DUT to power ON")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	// Verify battery is charging.
	s.Log("Check that DUT is charging")
	isAttached, err := pxy.Servo().GetChargerAttached(ctx)
	if err != nil {
		s.Fatal("Failed to check whether DUT is charging: ", err)
	}
	if !isAttached {
		s.Fatal("DUT is not charging after waking up from hibernation")
	}
	s.Logf("Charger attached: %t", isAttached)

	// Verify Power-state info.
	s.Log("Get power-state information")
	checkedValue, err := pxy.Servo().GetECSystemPowerState(ctx)
	if err != nil {
		s.Fatal("Failed to get power-state info: ", err)
	}
	if checkedValue != "S0" {
		s.Fatalf("Incorrect Power State after EC has woken up: %q", checkedValue)
	}
	s.Logf("Power State: %q", checkedValue)
}
