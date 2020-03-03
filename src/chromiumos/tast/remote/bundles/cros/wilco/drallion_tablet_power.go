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
		// on drallion360 devices when in tablet mode and requires a seperate test.
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

	// Inline function here ensures the deferred call to CloseChrome() runs before the DUT turned off
	func() {
		// Connect to the gRPC server on the DUT.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC: ", err)
		}
		defer cl.Close(ctx)
		powerMenuService := pb.NewPowerMenuServiceClient(cl.Conn)

		// Chrome instance is necessary to check for the presence of the power menu
		if _, err = powerMenuService.NewChrome(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to create new chrome instance: ", err)
		}
		defer powerMenuService.CloseChrome(ctx, &empty.Empty{})

		// Use servo to hold down power button for < 1.5 sec (clamshell mode)
		s.Log("Pressing power key for 0.5 seconds (clamshell mode)")
		if err = pxy.Servo().SetString(ctx, "power_key", "0.5"); err != nil {
			s.Fatal("Error pressing the power button: ", err)
		}

		// Verify that power down menu was triggered
		res, err := powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("RPC call failed: ", err)
		}
		if res.IsMenuPresent == false {
			s.Fatal("< 1.5 sec power key press failed to trigger the power menu in clamshell mode")
		}

		// Verify that boot ID has not changed
		curID, err := readBootID(ctx)
		if err != nil {
			s.Fatal("Failed to read boot ID: ", err)
		}
		if curID != initID {
			s.Fatal("DUT rebooted after short power key press in clamshell mode")
		}

		// Use servo to set tablet_mode_angle to 0 degrees (force device into tablet mode).
		out, err = d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", "0", "0").Output(ctx)
		if err != nil {
			s.Fatal("Failed to set tablet_mode_angle: ", err)
		}
		s.Logf("Put device into tablet mode: %s", out)

		// Use servo to hold down power button for < 1.5 sec
		s.Log("Pressing power key for 0.5 seconds")
		if err = pxy.Servo().SetString(ctx, "power_key", "0.5"); err != nil {
			s.Fatal("Error pressing the power button: ", err)
		}

		// Verify that power down menu was not triggered
		res, err = powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("RPC call failed: ", err)
		}
		if res.IsMenuPresent == true {
			s.Fatal("< 1.5 sec power key press triggered the power menu")
		}

		// Verify that boot ID has not changed
		curID, err := readBootID(ctx)
		if err != nil {
			s.Fatal("Failed to read boot ID: ", err)
		}
		if curID != initID {
			s.Fatal("DUT rebooted after short power key press")
		}

		// Use servo to hold down power button for > 1.5 sec
		s.Log("Pressing power key for 2 seconds")
		if err = pxy.Servo().SetString(ctx, "power_key", "2.0"); err != nil {
			s.Fatal("Error pressing the power button: ", err)
		}

		// Verify that power down menu was triggered
		res, err = powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("RPC call failed: ", err)
		}
		if res.IsMenuPresent == false {
			s.Fatal("> 1.5 sec power key press failed to trigger the power menu")
		}

		// Verify that boot ID has not changed
		curID, err = readBootID(ctx)
		if err != nil {
			s.Fatal("Failed to read boot ID: ", err)
		}
		if curID != initID {
			s.Fatal("DUT rebooted after power key press < 8 sec")
		}
	}()

	// Use servo to hold down power button for > 8 sec
	s.Log("Pressing power key for 8.5 seconds")
	if err = pxy.Servo().SetString(ctx, "power_key", "8.5"); err != nil {
		s.Fatal("Error pressing the power button: ", err)
	}

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

	// Verify that boot ID has changed
	curID, err := readBootID(ctx)
	if err != nil {
		s.Fatal("Failed to read boot ID: ", err)
	}
	if curID == initID {
		s.Fatal("Boot ID did not change")
	}
}
