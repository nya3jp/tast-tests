// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECHibernate,
		Desc:         "Checks that device will charge when EC is in a low-power mode, as a replacement for manual test 1.4.11",
		Contacts:     []string{"arthur.chuang@cienet.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		SoftwareDeps: []string{"crossystem"},
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
	if err = pxy.Servo().SetV4Role(ctx, servo.V4RoleSnk); err != nil {
		s.Fatal("Error disabling charge-through: ", err)
	}

	// Wait for a short delay between cutting power supply and telling EC to hibernate.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Tell EC to hibernate.
	s.Log("Put DUT in hibernatone mode")
	if err = pxy.Servo().RunECCommand(ctx, "hibernate"); err != nil {
		s.Fatal("Failed to hibernate: ", err)
	}

	// Wait for a short delay after putting DUT in hibernation.
	if err := testing.Sleep(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}

	// Verify that EC is non-responsive by sending a query on chargestate.
	s.Log("Verify EC is non-responsive")
	waitECCtx, cancelEC := context.WithTimeout(ctx, 10*time.Second)
	defer cancelEC()

	// Expect no return for the query, and timeout after 10 seconds.
	_, errEC := d.Command("ectool", "chargestate", "show").Output(waitECCtx)
	if !errors.As(errEC, &context.DeadlineExceeded) {
		s.Fatal("EC was still responsive after being told to hibernate")
	}
	s.Logf("EC was non-responsive: %q", errEC)

	// Wake EC up by telling servo to re-supply power.
	s.Log("Enable charge-through")
	if err = pxy.Servo().SetV4Role(ctx, servo.V4RoleSrc); err != nil {
		s.Fatal("Error enabling charge-through: ", err)
	}

	s.Log("Waiting for DUT to power ON")
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 50*time.Second)
	defer cancelWaitConnect()

	// Wait for DUT to reboot and reconnect.
	if err = d.WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	// Verify battery is charging.
	s.Log("Get charge-state information")
	re := regexp.MustCompile(`ac = (\d+)\Wchg_voltage = (\d+..)\Wchg_current = (\d+..)\Wchg_input_current = (\d+..)\Wbatt_state_of_charge = (\d+.)`)
	out, err := d.Command("ectool", "chargestate", "show").Output(ctx)
	if err != nil {
		s.Fatal("Failed to retreive charge-state info: ", err)
	}
	result := re.FindStringSubmatch(string(out))
	if len(result) != 6 {
		s.Fatal("Failed to get charge-state information")
	}
	if result[1] != "1" {
		s.Fatal("Battery is not charging")
	}
	s.Logf("ac = %q, chg_voltage = %q, chg_current = %q, chg_input_current = %q, batt_state_of_charge = %q",
		result[1], result[2], result[3], result[4], result[5])

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
