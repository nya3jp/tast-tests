// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/services/cros/wilco"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECTabletMode,
		Desc:         "Checks that power button actions behave as expected in tablet mode, replacing case 1.4.9",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_smoke"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService", "tast.cros.wilco.PowerMenuService", "tast.cros.graphics.ScreenshotService"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECTabletMode(ctx context.Context, s *testing.State) {

	d := s.DUT()

	readBootID := func(ctx context.Context) (string, error) {
		out, err := d.Command("cat", "/proc/sys/kernel/random/boot_id").Output(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to read boot id")
		}
		return strings.TrimSpace(string(out)), nil
	}

	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	// Get the initial tablet_mode_angle settings to set back at end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := d.Command("ectool", "motionsense", "tablet_mode_angle").Output(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	initLidAngle := re.FindStringSubmatch(string(out))
	if len(initLidAngle) != 3 {
		s.Fatal("Failed to get initial tablet_mode_angle settings")
	}
	s.Logf("Initial settings: lid_angle=%s hys=%s", initLidAngle[1], initLidAngle[2])

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	s.Log("Put DUT into tablet mode")
	_, err = d.Command("ectool", "motionsense", "tablet_mode_angle", "0", "0").Output(ctx)
	if err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	s.Log("Press power key to turn DUT OFF")
	if err = pxy.Servo().SetString(ctx, "power_key", "9"); err != nil {
		s.Fatal("Failed to press the power button to turn DUT off: ", err)
	}

	s.Log("Wait for DUT to power OFF")
	if err = d.WaitUnreachable(ctx); err != nil {
		s.Fatal("DUT did not power down after power key press > 8 seconds")
	}

	s.Log("Press power key to turn DUT ON")
	if err = pxy.Servo().SetString(ctx, "power_state", "on"); err != nil {
		s.Fatal(err, "Failed to send power key press to turn DUT ON: ", err)
	}

	s.Log("Wait for DUT to power ON")
	if err = d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	// Get initial boot ID.
	initID, err := readBootID(ctx)
	if err != nil {
		s.Fatal("Failed to read boot ID: ", err)
	}
	s.Logf("Initial boot ID: %s", initID)

	// Connect to the gRPC server on the DUT.
	cl, errRPC := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if errRPC != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	screenshotService := graphics.NewScreenshotServiceClient(cl.Conn)
	checkDisplay := func(ctx context.Context) error {
		if _, err := screenshotService.CaptureScreen(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		return nil
	}

	powerMenuService := wilco.NewPowerMenuServiceClient(cl.Conn)
	checkPowerMenuAfterLogin := func(ctx context.Context) error {
		resPowerMenu, errPowerMenu := powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
		if errPowerMenu != nil {
			return errors.Wrap(errPowerMenu, "failed to check power menu")
		}
		if resPowerMenu.IsMenuPresent != true {
			return errors.New("Power menu does not exist")
		}
		return nil
	}

	screenLockService := ui.NewScreenLockServiceClient(cl.Conn)
	lockScreen := func(ctx context.Context) error {
		if _, err := screenLockService.Lock(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to lock screen")
		}
		return nil
	}

	turnDisplayOffAndOn := func(ctx context.Context) error {
		s.Log("Turn display off then on, and check that display behaves as expected")
		actions := []string{"OFF", "ON"}
		for i := 0; i < len(actions); i++ {

			if err = pxy.Servo().SetString(ctx, "power_key", "0.5"); err != nil {
				return errors.Wrapf(err, "error pressing the power button for 0.5 second to turn display %s", actions[i])
			}

			// Wait for a short delay after sending the power key.
			if err := testing.Sleep(ctx, 1*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}

			switch {
			case actions[i] == "OFF":
				errDisplayOFF := checkDisplay(ctx)
				if !strings.Contains(errDisplayOFF.Error(), "CRTC not found. Is the screen on?") {
					return errors.Wrap(err, "display was not turned OFF")
				}
			case actions[i] == "ON":
				if err = checkDisplay(ctx); err != nil {
					return errors.Wrap(err, "display was not turned ON")
				}
			}
		}
		return nil
	}

	repeatedSteps := func(testCase string) {

		if testCase == "atSignin" {
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
		}
		if testCase == "afterSignin" {
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Chrome instance is necessary to check for the presence of the power menu.
			if _, err = powerMenuService.NewChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to create new chrome instance for powerMenuService: ", err)
			}
		}
		if testCase == "atLockScreen" {
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Reuse the existing chrome instance created by powerMenuService for screenLockService.
			if _, err = screenLockService.ReuseChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to reuse existing chrome session for screenLockService: ", err)
			}
			s.Log("Lock Screen")
			if err = lockScreen(ctx); err != nil {
				s.Fatal("Lock-screen did not behave as expected: ", err)
			}
			// Close chrome instance at the end of the test.
			defer powerMenuService.CloseChrome(ctx, &empty.Empty{})
		}

		if err = turnDisplayOffAndOn(ctx); err != nil {
			s.Fatal("Display did not behave as expected: ", err)
		}

		s.Log("Press power key for 1 second to bring up the power menu")
		if err = pxy.Servo().SetString(ctx, "power_key", "1"); err != nil {
			s.Fatal("Failed to press the power button for 1 second to bring up the power menu: ", err)
		}

		// Wait for a short delay after sending the power key.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		switch {
		case testCase == "atSignin":
			s.Log("***********Work in progress: API pending for powerMenuService at the sign in screen***********")
		case testCase == "afterSignin", testCase == "atLockScreen":
			if err := checkPowerMenuAfterLogin(ctx); err != nil {
				s.Fatal("Power menu did not behave as expected: ", err)
			}
			s.Log("Power menu exists")
		}

		if err = turnDisplayOffAndOn(ctx); err != nil {
			s.Fatal("Display did not behave as expected: ", err)
		}

		// Press power button for 2.5 seconds to activate the pre-shutdown animation.
		s.Log("Press power key for 2.5 seconds to activate the pre-shutdown animation")
		if err = pxy.Servo().SetString(ctx, "power_key", "2.5"); err != nil {
			s.Fatal("Failed to press the power button for 2.5 seconds to activate the pre-shutdown animation: ", err)
		}

		// Wait for a short delay after sending the power key.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		// Verify that DUT did not reboot.
		curID, errBootID := readBootID(ctx)
		if errBootID != nil {
			s.Fatal("Failed to read boot ID: ", errBootID)
		}
		if curID != initID {
			s.Fatal("DUT rebooted after short power press for 2.5 seconds")
		}
		s.Log("DUT did not reboot")
	}

	var testCases = []string{"atSignin", "afterSignin", "atLockScreen"}
	for _, testCase := range testCases {
		repeatedSteps(testCase)
	}

	_, err = d.Command("ectool", "motionsense", "tablet_mode_angle", initLidAngle[1], initLidAngle[2]).Output(ctx)
	if err != nil {
		s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
	}
	s.Logf("Restore tablet_mode_angle to the original settings: lid_angle=%s hys=%s", initLidAngle[1], initLidAngle[2])
}
