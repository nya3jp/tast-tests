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
		// TODO(mwiitala): Restore attributes after fixing http://b/149035007
		// Attr: []string{ "group:mainline", "informational"},
		Vars: []string{"servo"},
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
	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
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
	// Restore tablet_mode_angle settings before returning
	defer d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", initLidAngle[1], initLidAngle[2]).Run(ctx)

	setTabletMode := func(tabletMode bool) error {
		// Setting tabletModeAngle to 0 will force the DUT into tablet mode
		tabletModeAngle := "0"
		mode := "tablet"
		if !tabletMode {
			// Setting tabletModeAngle to 360 will force the DUT into clamshell mode
			tabletModeAngle = "360"
			mode = "clamshell"
		}
		// Use servo to set tablet_mode_angle
		out, err = d.Command("ectool", "--name=cros_ish", "motionsense", "tablet_mode_angle", tabletModeAngle, "0").Output(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to set tablet_mode_angle")
		}
		s.Logf("Put DUT into %s mode", mode)
		return nil
	}

	// Press power key for pressDuration seconds and verify DUT reboots as expected
	testCaseReboot := func(pressDuration string) error {
		// Restarting Chrome clears the power down menu if already present
		if _, err = powerMenuService.NewChrome(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to create new chrome instance")
		}
		// Close chrome instance before rebooting
		if _, err = powerMenuService.CloseChrome(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to close chrome instance")
		}

		// Use servo to hold down power button
		s.Logf("Pressing power key for %s seconds", pressDuration)
		if err = pxy.Servo().SetString(ctx, "power_key", pressDuration); err != nil {
			return errors.Wrap(err, "error pressing the power button")
		}

		waitUnreachableCtx, cancelUnreachable := context.WithTimeout(ctx, 30*time.Second)
		defer cancelUnreachable()

		// Wait for DUT to power off as expected
		s.Log("Waiting for DUT to power OFF")
		if err = d.WaitUnreachable(waitUnreachableCtx); err != nil {
			return errors.New("DUT did not power down after power key press > 8 seconds")
		}

		// Use servo to power DUT on
		s.Log("Sending power key press to turn DUT back on")
		if err = pxy.Servo().SetString(ctx, "power_state", "on"); err != nil {
			return errors.Wrap(err, "failed to send power key press")
		}

		s.Log("Waiting for DUT to power ON")
		waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWaitConnect()

		// Wait for DUT to reboot and reconnect
		if err = d.WaitConnect(waitConnectCtx); err != nil {
			return errors.Wrap(err, "failed to reconnect to DUT")
		}

		// Verify that DUT rebooted
		curID, err := readBootID(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read boot ID")
		}
		if curID == initID {
			return errors.Errorf("DUT failed to reboot after power key press of %s seconds", pressDuration)
		}
		// Update initID for following test cases
		initID = curID

		// Reconnect to the gRPC server on the DUT for following test cases
		cl, err = rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			return errors.Wrap(err, "failed to connect to the RPC")
		}
		powerMenuService = pb.NewPowerMenuServiceClient(cl.Conn)

		return nil
	}

	// Press power key for pressDuration seconds, check that power menu only appears if expected, confirm DUT did not reboot
	testCaseNoReboot := func(pressDuration string, menuExpected bool) error {
		// Chrome instance is necessary to check for the presence of the power menu
		if _, err = powerMenuService.NewChrome(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to create new chrome instance")
		}
		defer powerMenuService.CloseChrome(ctx, &empty.Empty{})

		// Use servo to hold down power button
		s.Logf("Pressing power key for %s seconds", pressDuration)
		if err = pxy.Servo().SetString(ctx, "power_key", pressDuration); err != nil {
			return errors.Wrap(err, "error pressing the power button")
		}

		// Verify that power down menu is only present when expected
		res, err := powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			return errors.Wrap(err, "RPC call failed")
		}
		if res.IsMenuPresent != menuExpected {
			return errors.Errorf("Power menu did not behave as expected after pressing power key for %s seconds", pressDuration)
		}

		// Verify that DUT did not reboot
		curID, err := readBootID(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to read boot ID")
		}
		if curID != initID {
			return errors.Errorf("DUT rebooted after power key press of %s seconds", pressDuration)
		}
		return nil
	}

	/* Iterate over test cases and verify expected behavior:
	Clamshell mode: Power menu appears after power key press of any duration
	Tablet mode: Power menu only appears after power key press > 1.5 seconds
	DUT powers down after power key press > 8.0 seconds in both modes
	*/
	for _, tc := range []struct {
		tabletMode     bool
		pressDuration  string
		menuExpected   bool
		rebootExpected bool
	}{
		{false, "0.5", true, false},
		{true, "0.5", false, false},
		{false, "2.0", true, false},
		{true, "2.0", true, false},
		{false, "8.5", true, true},
		{true, "8.5", true, true},
	} {
		// Use servo to force DUT into tablet or clamshell mode
		if err := setTabletMode(tc.tabletMode); err != nil {
			s.Fatal("Failed to set tablet mode angel: ", err)
		}

		// Verify test case expectations
		if !tc.rebootExpected {
			err = testCaseNoReboot(tc.pressDuration, tc.menuExpected)
		} else {
			err = testCaseReboot(tc.pressDuration)
		}

		if err != nil {
			s.Fatalf("Failed on test case with tabletMode=%t, pressDuration=%s, menuExpected=%t, rebootExpected=%t: %v",
				tc.tabletMode, tc.pressDuration, tc.menuExpected, tc.rebootExpected, err)
		}
	}
}
