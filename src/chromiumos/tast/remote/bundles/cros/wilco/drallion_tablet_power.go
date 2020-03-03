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

	// Return early if DUT is not a Drallion device
	out, err := d.Command("cat", "/etc/lsb-release").Output(ctx)
	if err != nil {
		s.Fatal("Failed to check Drallion model: ", err)
	}
	if !strings.Contains(string(out), "CHROMEOS_RELEASE_BOARD=drallion") {
		s.Fatal("DUT is not a Drallion device, returning early")
	}

	// This is expected to fail in VMs, since Servo is unusable there and the "servo" var won't
	// be supplied. https://crbug.com/967901 tracks finding a way to skip tests when needed.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Get the initial tablet_mode_angle settings to set back at end of test
	re := regexp.MustCompile(`tablet_mode_angle=([\d]+) hys=([\d]+)`)
	out, err = d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle").Output(ctx)
	if err != nil || !re.MatchString(string(out)) {
		s.Fatal("Failed to get initial tablet_mode_angle settings")
	}
	initLidAngle := re.FindStringSubmatch(string(out))
	s.Logf("Initial settings: lid_angle=%s hys=%s", initLidAngle[1], initLidAngle[2])

	// Use servo to set tablet_mode_angle to 0 degrees (Device will always be in tablet mode).
	out, err = d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", "0", "0").Output(ctx)
	if err != nil {
		s.Fatal("Failed to set tablet_mode_angle: ", err)
	}
	s.Logf("%s", out)
	// Restore tablet_mode_angle settings before returning
	defer d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", initLidAngle[0], initLidAngle[1]).Run(ctx)

	// Get initial boot ID
	initID, err := readBootID(ctx)
	if err != nil {
		s.Fatal("Failed to read boot ID: ", err)
	}
	s.Logf("Initial boot ID: %s", initID)

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
		_, err = powerMenuService.NewChrome(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to create new chrome instance: ", err)
		}
		defer powerMenuService.CloseChrome(ctx, &empty.Empty{})

		// Use servo to hold down power button for < 1.5 sec
		s.Log("Pressing power key for 0.5 seconds")
		err = pxy.Servo().SetString(ctx, "power_key", "0.5")
		if err != nil {
			s.Fatal("Error pressing the power button: ", err)
		}

		// Verify that power down menu was not triggered
		res, err := powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
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
		err = pxy.Servo().SetString(ctx, "power_key", "2.0")
		if err != nil {
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
	err = pxy.Servo().SetString(ctx, "power_key", "8.5")
	if err != nil {
		s.Fatal("Error pressing the power button: ", err)
	}

	waitUnreachableCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Wait for DUT to power off as expected
	s.Log("Waiting for DUT to power OFF")
	err = d.WaitUnreachable(waitUnreachableCtx)
	if err != nil {
		s.Fatal("DUT did not power down after power key press > 8 seconds")
	}

	// Use servo to power DUT on
	s.Log("Sending power key press to turn DUT back on")
	err = pxy.Servo().SetString(ctx, "power_state", "on")
	if err != nil {
		s.Fatal("Failed to send power key press: ", err)
	}

	s.Log("Waiting for DUT to power ON")
	waitConnectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Wait for DUT to reboot and reconnect
	err = d.WaitConnect(waitConnectCtx)
	if err != nil {
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
