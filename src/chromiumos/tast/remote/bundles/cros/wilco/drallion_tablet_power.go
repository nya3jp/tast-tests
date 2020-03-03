// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DrallionTabletPower,
		Desc: "Verifies power button behavior on Drallion 360 devices in tablet mode",
		Contacts: []string{
			"mwiitala@google.com", // Author
			"tast-owners@google.com",
		},
		SoftwareDeps: []string{"wilco", "tablet_mode", "chrome"},
		ServiceDeps:  []string{"tast.cros.wilco.PowerMenuService"},
		// On Drallion360, the power button is on the keyboard rather than the side
		// of the device. To account for this, the power button behaves differently
		// on drallion360 devices when in tablet mode and requires a separate test.
		HardwareDeps: hwdep.D(hwdep.Model("drallion360")),
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"servo"},
	})
}

func DrallionTabletPower(ctx context.Context, s *testing.State) {
	d := s.DUT()
	readBootID := func(ctx context.Context) (string, error) {
		out, err := d.Command("cat", "/proc/sys/kernel/random/boot_id").Output(ctx)
		if err != nil {
			return "", errors.Wrap(err, "error reading boot id")
		}
		return strings.TrimSpace(string(out)), nil
	}

	// This is expected to fail in VMs, since Servo is unusable there and the "servo" var won't
	// be supplied. https://crbug.com/967901 tracks finding a way to skip tests when needed.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC: ", err)
	}
	defer cl.Close(ctx)
	powerMenuService := pb.NewPowerMenuServiceClient(cl.Conn)

	// Get initial boot ID
	initID, err := readBootID(ctx)
	if err != nil {
		s.Fatal("Failed to read boot ID: ", err)
	}
	s.Logf("Initial boot ID: %s", initID)

	// Get the initial tablet_mode_angle settings to set back at end of test
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle").Output(ctx)
	if err != nil {
		s.Fatal("Failed to retreive tablet_mode_angle settings: ", err)
	}
	initLidAngle := re.FindStringSubmatch(string(out))
	if len(initLidAngle) != 3 {
		s.Fatal("Failed to get initial tablet_mode_angle settings")
	}
	s.Logf("Initial settings: lid_angle=%s hys=%s", initLidAngle[1], initLidAngle[2])

	// Use servo to set tablet_mode_angle to 360 degrees (force device into clamshell mode).
	out, err = d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", "360", "0").Output(ctx)
	if err != nil {
		s.Fatal("Failed to set tablet_mode_angle: ", err)
	}
	s.Logf("Put device into clamshell mode: %s", out)
	// Restore tablet_mode_angle settings before returning
	defer d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", initLidAngle[0], initLidAngle[1]).Run(ctx)

	// First test cases to run verify clamshell mode behavior
	tabletMode := false

	for _, tc := range []struct {
		tabletMode     bool
		pressDuration  string
		menuExpected   bool
		rebootExpected bool
	}{
		{false, "0.5", true, false},
		// Only tablet mode test cases below
		{true, "0.5", false, false},
		{true, "2.0", true, false},
		{true, "8.5", true, true},
	} {
		if tc.tabletMode && !tabletMode {
			// Use servo to set tablet_mode_angle to 0 degrees (force device into tablet mode).
			out, err = d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", "0", "0").Output(ctx)
			if err != nil {
				s.Fatal("Failed to set tablet_mode_angle: ", err)
			}
			s.Logf("Put device into tablet mode: %s", out)
			// Restore tablet_mode_angle settings before returning
			defer d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", initLidAngle[0], initLidAngle[1]).Run(ctx)

			tabletMode = true
		}

		if !tc.rebootExpected {
			// Chrome instance is necessary to check for the presence of the power menu
			if _, err = powerMenuService.NewChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to create new chrome instance: ", err)
			}
			defer powerMenuService.CloseChrome(ctx, &empty.Empty{})
		}

		// Use servo to hold down power button
		s.Logf("Pressing power key for %s seconds", tc.pressDuration)
		if err = pxy.Servo().SetString(ctx, "power_key", tc.pressDuration); err != nil {
			s.Fatal("Error pressing the power button: ", err)
		}

		if !tc.rebootExpected {
			// Verify that power down menu is only present when expected
			res, err := powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
			if err != nil {
				s.Fatal("RPC call failed: ", err)
			}
			if res.IsMenuPresent != tc.menuExpected {
				s.Fatalf("Power menu did not behave as expected after pressing power key for %s seconds", tc.pressDuration)
			}

			// Close chrome instance
			if _, err = powerMenuService.CloseChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to close chrome instance: ", err)
			}
		} else {
			// Reboot expected
			waitUnreachableCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Wait for DUT to power off as expected
			s.Log("Waiting for DUT to power OFF")
			if err = d.WaitUnreachable(waitUnreachableCtx); err != nil {
				s.Fatal("DUT did not power down after power key press > 8 seconds")
			}

			// Use servo to power DUT on
			s.Log("Sending power key press to turn DUT back on")
			if err = pxy.Servo().SetString(ctx, "power_state", "on"); err != nil {
				s.Fatal("Failed to send power key press: ", err)
			}

			s.Log("Waiting for DUT to power ON")
			waitConnectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Wait for DUT to reboot and reconnect
			if err = d.WaitConnect(waitConnectCtx); err != nil {
				s.Fatal("Failed to reconnect to DUT: ", err)
			}

			// DUT may not be in tablet mode after reboot
			tabletMode = false
		}

		// Verify that boot ID has only changed if expected
		curID, err := readBootID(ctx)
		if err != nil {
			s.Fatal("Failed to read boot ID: ", err)
		}
		if !tc.rebootExpected && curID != initID {
			s.Fatalf("DUT rebooted after power key press of %s seconds", tc.pressDuration)
		}
		if tc.rebootExpected {
			if curID == initID {
				s.Fatalf("DUT failed to reboot after power key press of %s seconds", tc.pressDuration)
			}
			// Update initID for following test cases
			initID = curID
		}
	}
}
