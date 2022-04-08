// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/remote/firmware/reporters"
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that power button actions behave as expected in tablet mode, replacing case 1.4.9",
		Contacts:     []string{"arthur.chuang@cienet.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_ec"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenLockService", "tast.cros.ui.PowerMenuService", "tast.cros.graphics.ScreenshotService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.FormFactor(hwdep.Convertible, hwdep.Chromeslate, hwdep.Detachable)),
	})
}

func ECTabletMode(ctx context.Context, s *testing.State) {
	d := s.DUT()

	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	// Run EC command to put DUT in tablet mode.
	if err := h.Servo.RunECCommand(ctx, "tabletmode on"); err != nil {
		s.Fatal("Failed to set DUT into tablet mode: ", err)
	}

	defer func() {
		if err := h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
			s.Fatal("Failed to restore DUT to the original tabletmode setting: ", err)
		}
	}()

	s.Log("Power-cycle DUT with a warm reset")
	h.CloseRPCConnection(ctx)
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateWarmReset); err != nil {
		s.Fatal("Failed to reboot DUT by servo: ", err)
	}

	s.Log("Wait for DUT to power ON")
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnect()

	if err := d.WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	// Get initial boot ID.
	r := reporters.New(d)
	origID, err := r.BootID(ctx)
	if err != nil {
		s.Fatal("Failed to read the original boot ID: ", err)
	}

	// Connect to the RPC service on the DUT.
	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// The checkDisplay function checks whether display is on/off
	// by attempting to capture a screenshot. If capturing a screenshot fails,
	// the returned stderr message, "CRTC not found. Is the screen on?", would
	// be returned and checked. Also, in this case, since the screenshot file
	// saved is not needed, it would always get deleted immediately.
	screenshotService := graphics.NewScreenshotServiceClient(h.RPCClient.Conn)
	checkDisplay := func(ctx context.Context) error {
		if _, err := screenshotService.CaptureScreenAndDelete(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to take screenshot")
		}
		return nil
	}

	// The checkPowerMenu function will check whether the power menu is present
	// after holding the power button for about one second.
	powerMenuService := ui.NewPowerMenuServiceClient(h.RPCClient.Conn)
	checkPowerMenu := func(ctx context.Context) error {
		res, err := powerMenuService.PowerMenuPresent(ctx, &empty.Empty{})
		if err != nil {
			return errors.Wrap(err, "failed to check power menu")
		}
		if !res.IsMenuPresent {
			return errors.New("power menu does not exist")
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
		for _, turnOn := range []bool{false, true} {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab); err != nil {
					return errors.Wrap(err, "error pressing power_key:tab")
				}

				if err := testing.Sleep(ctx, 1*time.Second); err != nil {
					return errors.Wrap(err, "error in sleeping for 1 second after pressing on the power key")
				}

				switch turnOn {
				case false:
					err := checkDisplay(ctx)
					if err == nil {
						return errors.New("unexpectedly able to take screenshot after setting display power off")
					}
					if !strings.Contains(err.Error(), "CRTC not found. Is the screen on?") {
						return errors.Wrap(err, "unexpected error when taking screenshot")
					}
				case true:
					if err := checkDisplay(ctx); err != nil {
						return errors.Wrap(err, "display was not turned ON")
					}
				}
				return nil
			}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to set display on/off")
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
			// Reuse the existing login session from same user.
			if _, err := screenLockService.ReuseChrome(ctx, &empty.Empty{}); err != nil {
				s.Fatal("Failed to reuse existing chrome session for screenLockService: ", err)
			}
			s.Log("Lock Screen")
			if err := lockScreen(ctx); err != nil {
				s.Fatal("Lock-screen did not behave as expected: ", err)
			}
			// Close chrome instance at the end of the test.
			defer screenLockService.CloseChrome(ctx, &empty.Empty{})
		}

		if err := turnDisplayOffAndOn(ctx); err != nil {
			s.Fatal("Display did not behave as expected: ", err)
		}

		s.Log("Bring up the power menu with power_key:press")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				return errors.Wrap(err, "error bringing up the power menu")
			}
			if err := checkPowerMenu(ctx); err != nil {
				return errors.Wrapf(err, "power menu did not behave as expected %s", testCase)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			s.Fatal("Failed to check whether the power menu is present: ", err)
		}
		s.Logf("Power menu exists: %s", testCase)

		if err := turnDisplayOffAndOn(ctx); err != nil {
			s.Fatal("Display did not behave as expected: ", err)
		}

		// Short press on power button to activate the pre-shutdown animation.
		s.Log("Activate the pre-shutdown animation")
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.Dur((h.Config.HoldPwrButtonPowerOff)/3)); err != nil {
			s.Fatal("Failed to set a KeypressControl by servo: ", err)
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
