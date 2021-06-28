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
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	atSignin     = "atSignin"
	afterSignin  = "afterSignin"
	atLockScreen = "atLockScreen"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECTabletMode,
		Desc:         "Checks that power button actions behave as expected in tablet mode, replacing case 1.4.9",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: []string{"chrome", "crossystem", "flashrom"},
		Data:         []string{firmware.ConfigFile},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService", "tast.cros.ui.ScreenLockService", "tast.cros.ui.PowerMenuService", "tast.cros.graphics.ScreenshotService"},
		Pre:          pre.DevMode(),
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func ECTabletMode(ctx context.Context, s *testing.State) {

	d := s.DUT()

	h := s.PreValue().(*pre.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	// Get the initial tablet_mode_angle settings to restore at the end of test.
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := d.Command("ectool", "motionsense", "tablet_mode_angle").Output(ctx)
	if err != nil {
		s.Fatal("Failed to retrieve tablet_mode_angle settings: ", err)
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		s.Fatalf("Failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := m[1]
	initHys := m[2]
	s.Logf("Initial settings: lid_angle=%q hys=%q", initLidAngle, initHys)

	defer func() {
		if err := d.Command("ectool", "motionsense", "tablet_mode_angle", string(initLidAngle), string(initHys)).Run(ctx); err != nil {
			s.Fatal("Failed to restore tablet_mode_angle to the original settings: ", err)
		}
	}()

	// Set tabletModeAngle to 0 to force the DUT into tablet mode.
	s.Log("Put DUT into tablet mode")
	if err := d.Command("ectool", "motionsense", "tablet_mode_angle", "0", "0").Run(ctx); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	s.Log("Press power key to turn DUT OFF")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOff); err != nil {
		s.Fatal("Failed to set a KeypressControl by servo: ", err)
	}

	s.Log("Wait for DUT to power OFF")
	waitUnreachableCtx, cancelUnreachable := context.WithTimeout(ctx, 1*time.Minute)
	defer cancelUnreachable()

	if err := d.WaitUnreachable(waitUnreachableCtx); err != nil {
		s.Fatal("DUT did not power down: ", err)
	}

	s.Log("Press power key to turn DUT ON")
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateOn); err != nil {
		s.Fatal("Failed to reboot DUT by servo: ", err)
	}

	s.Log("Wait for DUT to power ON")
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnect()

	if err := d.WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	defer h.CloseRPCConnection(ctx)

	// Get initial boot ID.
	r := reporters.New(d)
	origID, err := r.BootID(ctx)
	if err != nil {
		s.Fatal("Failed to read the original boot ID: ", err)
	}
	s.Logf("Initial boot ID: %s", origID)

	// Connect to the RPC service on the DUT.
	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	screenshotService := graphics.NewScreenshotServiceClient(h.RPCClient.Conn)
	checkDisplay := func(ctx context.Context) error {
		if _, err := screenshotService.CaptureScreenAndDelete(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		return nil
	}

	powerMenuService := ui.NewPowerMenuServiceClient(h.RPCClient.Conn)
	checkPowerMenu := func(ctx context.Context) error {
		res, err := powerMenuService.IsPowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			return errors.Wrap(err, "failed to check power menu")
		}
		if !res.IsMenuPresent {
			return errors.New("Power menu does not exist")
		}
		return nil
	}

	screenLockService := ui.NewScreenLockServiceClient(h.RPCClient.Conn)
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

			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
				return errors.Wrapf(err, "error turning display %s with power_key:tab", actions[i])
			}

			// Wait for a short delay after sending the power key.
			if err := testing.Sleep(ctx, 1*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}

			switch actions[i] {
			case "OFF":
				err := checkDisplay(ctx)
				if !strings.Contains(err.Error(), "CRTC not found. Is the screen on?") {
					return errors.Wrap(err, "display was not turned OFF")
				}
			case "ON":
				if err := checkDisplay(ctx); err != nil {
					return errors.Wrap(err, "display was not turned ON")
				}
			}
		}
		return nil
	}

	repeatedSteps := func(testCase string) {
		switch testCase {
		case atSignin:
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Chrome instance is necessary to check for the presence of the power menu.
			// Start Chrome to show the login screen with a user pod.
			manifestKey := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
			signInRequest := ui.NewChromeRequest{
				Login: false,
				Key:   manifestKey,
			}
			if _, err := powerMenuService.NewChrome(ctx, &signInRequest); err != nil {
				s.Fatal("Failed to create new chrome instance with no login for powerMenuService: ", err)
			}
			// Close chrome instance and restart one again with login.
			defer powerMenuService.CloseChrome(ctx, &empty.Empty{})

		case afterSignin:
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Start Chrome and log in as testuser.
			signInRequest := ui.NewChromeRequest{
				Login: true,
				Key:   "",
			}
			if _, err := powerMenuService.NewChrome(ctx, &signInRequest); err != nil {
				s.Fatal("Failed to create new chrome instance with login for powerMenuService: ", err)
			}

		case atLockScreen:
			s.Logf("------------------------Perform testCase: %s------------------------", testCase)
			// Reuse the existing chrome instance created by powerMenuService for screenLockService.
			if _, err := screenLockService.ReuseChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to reuse existing chrome session for screenLockService: ", err)
			}
			s.Log("Lock Screen")
			if err := lockScreen(ctx); err != nil {
				s.Fatal("Lock-screen did not behave as expected: ", err)
			}
			// Close chrome instance at the end of the test.
			defer powerMenuService.CloseChrome(ctx, &empty.Empty{})
		}

		if err := turnDisplayOffAndOn(ctx); err != nil {
			s.Fatal("Display did not behave as expected: ", err)
		}

		s.Log("Bring up the power menu with power_key:press")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to bring up the power menu: ", err)
		}

		// Wait for a short delay after sending the power key.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		if err := checkPowerMenu(ctx); err != nil {
			s.Fatalf("Power menu did not behave as expected %s: %v", testCase, err)
		}
		s.Logf("Power menu exists: %s", testCase)

		if err := turnDisplayOffAndOn(ctx); err != nil {
			s.Fatal("Display did not behave as expected: ", err)
		}

		// Short press on power button to activate the pre-shutdown animation.
		s.Log("Activate the pre-shutdown animation")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur((h.Config.HoldPwrButtonPowerOff)/2)); err != nil {
			s.Fatal("Failed to set a KeypressControl by servo: ", err)
		}

		// Wait for a short delay after sending the power key.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}

		// Verify that DUT did not reboot.
		curID, err := r.BootID(ctx)
		if err != nil {
			s.Fatal("Failed to read the current boot ID: ", curID)
		}
		if curID != origID {
			s.Fatal("DUT rebooted after short power press")
		}
		s.Log("DUT did not reboot")
	}

	var testCases = []string{atSignin, afterSignin, atLockScreen}
	for _, testCase := range testCases {
		repeatedSteps(testCase)
	}
}
