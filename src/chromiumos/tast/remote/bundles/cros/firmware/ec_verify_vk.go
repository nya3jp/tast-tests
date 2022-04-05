// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	fwpb "chromiumos/tast/services/cros/firmware"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParamsTablet struct {
	canDoTabletSwitch bool
	formFactor        string
	tabletModeOn      string
	tabletModeOff     string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECVerifyVK,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify whether virtual keyboard window is present during change in tablet mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.CheckVirtualKeyboardService", "tast.cros.firmware.UtilsService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.TouchScreen()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				formFactor:        "convertible",
				tabletModeOn:      "tabletmode on",
				tabletModeOff:     "tabletmode off",
			},
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable)),
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				formFactor:        "detachable",
				tabletModeOn:      "basestate detach",
				tabletModeOff:     "basestate attach",
			},
		}, {
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val: &testParamsTablet{
				canDoTabletSwitch: false,
				formFactor:        "chromeslate",
			},
		}},
	})
}

func ECVerifyVK(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// Perform a hard reset on DUT to ensure removal of any
	// old settings that might potentially have an impact on
	// this test.
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		s.Fatal("Failed to cold reset DUT at the beginning of test: ", err)
	}

	// Wait for DUT to reconnect.
	waitConnectCtx, cancelWaitConnect := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelWaitConnect()

	if err := s.DUT().WaitConnect(waitConnectCtx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	cvkc := pb.NewCheckVirtualKeyboardServiceClient(h.RPCClient.Conn)
	s.Log("Starting a new Chrome session and logging in as test user")
	if _, err := cvkc.NewChromeLoggedIn(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to login: ", err)
	}
	defer cvkc.CloseChrome(ctx, &empty.Empty{})

	s.Log("Opening a Chrome page")
	if _, err := cvkc.OpenChromePage(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to open chrome: ", err)
	}

	// Create a Chrome instance for the utilsService by reusing one that's
	// already been created above under cvkc. The utilsService is required
	// by EvalTabletMode in checking tablet mode status.
	utilsService := fwpb.NewUtilsServiceClient(h.RPCClient.Conn)
	if _, err := utilsService.ReuseChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to reuse Chrome session for the utils service: ", err)
	}

	args := s.Param().(*testParamsTablet)
	// Chromeslates are already in tablet mode, and for this reason,
	// we could skip switching to tablet mode, and just verify that
	// virtual keyboard is present after a click on the address bar.
	if args.canDoTabletSwitch == false {
		if err := verifyVKIsPresent(ctx, h, cvkc, utilsService, s, true, "", args.formFactor); err != nil {
			s.Fatal("Failed to verify virtual keyboard status: ", err)
		}
	} else {
		for _, dut := range []struct {
			tabletMode  bool
			tabletState string
		}{
			{true, args.tabletModeOn},
			{false, args.tabletModeOff},
		} {
			s.Logf("Run test with tablet mode on: %t", dut.tabletMode)
			if err := verifyVKIsPresent(ctx, h, cvkc, utilsService, s, dut.tabletMode, dut.tabletState, args.formFactor); err != nil {
				s.Fatal("Failed to verify virtual keyboard status: ", err)
			}
		}
	}
}

func checkAndSetTabletMode(ctx context.Context, h *firmware.Helper, s *testing.State, action string) error {
	// regular expressions.
	var (
		tabletmodeNotFound = `Command 'tabletmode' not found or ambiguous`
		tabletmodeStatus   = `\[\S+ tablet mode (enabled|disabled)\]`
		basestateNotFound  = `Command 'basestate' not found or ambiguous`
		basestateStatus    = `\[\S+ base state: (attached|detached)\]`
		bdStatus           = `\[\S+ BD forced (connected|disconnected)\]`
		checkTabletMode    = `(` + tabletmodeNotFound + `|` + tabletmodeStatus + `|` + basestateNotFound + `|` + basestateStatus + `|` + bdStatus + `)`
	)
	// Run EC command to turn on/off tablet mode.
	s.Logf("Check command %q exists", action)
	out, err := h.Servo.RunECCommandGetOutput(ctx, action, []string{checkTabletMode})
	if err != nil {
		return errors.Wrapf(err, "failed to run command %q", action)
	}
	tabletModeUnavailable := []*regexp.Regexp{regexp.MustCompile(tabletmodeNotFound), regexp.MustCompile(basestateNotFound)}
	for _, v := range tabletModeUnavailable {
		if match := v.FindStringSubmatch(out[0][0]); match != nil {
			return errors.Errorf("device does not support tablet mode: %q", match)
		}
	}
	s.Logf("Current tabletmode status: %q", out[0][1])
	return nil
}

func verifyVKIsPresent(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, utilsService fwpb.UtilsServiceClient, s *testing.State, tabletMode bool, command, dutFormFactor string) error {
	// Run EC command to put DUT in clamshell/tablet mode.
	if command != "" {
		if err := checkAndSetTabletMode(ctx, h, s, command); err != nil {
			if dutFormFactor == "convertible" {
				s.Logf("Failed to set DUT tablet mode state, and got: %v. Attempting to set tablet_mode_angle with ectool instead", err)
				cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
				// Save initial tablet mode angle settings to restore at the end of verifyVKIsPresent.
				tabletModeAngleInit, hysInit, err := cmd.SaveTabletModeAngles(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to save initial tablet mode angles")
				}
				defer func() error {
					s.Logf("Restoring DUT's tablet mode angles to the original settings: lid_angle=%s, hys=%s", tabletModeAngleInit, hysInit)
					if err := cmd.ForceTabletModeAngle(ctx, tabletModeAngleInit, hysInit); err != nil {
						return errors.Wrap(err, "failed to restore tablet mode angle to the initial angles")
					}
					return nil
				}()
				if tabletMode {
					// Setting tabletModeAngle to 0s will force DUT into tablet mode.
					if err := cmd.ForceTabletModeAngle(ctx, "0", "0"); err != nil {
						return errors.Wrap(err, "failed to force DUT into tablet mode")
					}
				} else {
					// Setting tabletModeAngle to 360 will force DUT into clamshell mode.
					if err := cmd.ForceTabletModeAngle(ctx, "360", "0"); err != nil {
						return errors.Wrap(err, "failed to force DUT into clamshell mode")
					}
				}
			} else {
				return errors.Wrap(err, "failed to set DUT tablet mode state")
			}
		}
	}
	// Wait for the command on switching to tablet mode to fully propagate,
	// before clicking on the address bar.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed in sleeping for one second before clicking on the address bar")
	}

	// Log tablet mode status from the ChromeOS perspective.
	res, err := utilsService.EvalTabletMode(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "unable to evaluate whether ChromeOS is in tablet mode")
	}
	s.Logf("ChromeOS in tabletmode: %t", res.TabletModeEnabled)

	req := pb.CheckVirtualKeyboardRequest{
		IsDutTabletMode: tabletMode,
	}
	// Use polling here to wait till the UI tree has fully updated,
	// and check if virtual keyboard is present.
	s.Logf("Expecting virtual keyboard present: %t", tabletMode)
	checkVKPollOptions := testing.PollOptions{
		Timeout:  1 * time.Minute,
		Interval: 3 * time.Second,
	}
	if err := testing.Poll(ctx, func(c context.Context) error {
		s.Log("Sending a touch on the address bar of a Chrome page")
		if _, err := cvkc.TouchChromeAddressBar(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to touch on chrome address bar")
		}

		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep after clicking on the Chrome address bar")
		}

		res, err := cvkc.CheckVirtualKeyboardIsPresent(ctx, &req)
		if err != nil {
			return errors.Wrap(err, "failed to check whether virtual keyboard is present")
		}
		if tabletMode != res.IsVirtualKeyboardPresent {
			return errors.Errorf(
				"found unexpected behavior, and got tabletmode: %t, VirtualKeyboardPresent: %t",
				tabletMode, res.IsVirtualKeyboardPresent)
		}
		return nil
	}, &checkVKPollOptions); err != nil {
		s.Logf("Checking virtual keyboard in tablet mode failed with a tap on the touch screen: %v. Attempting a left click instead", err)
		return testing.Poll(ctx, func(ctx context.Context) error {
			s.Log("Clicking on the address bar of a Chrome page")
			if _, err := cvkc.ClickChromeAddressBar(ctx, &empty.Empty{}); err != nil {
				return errors.Wrap(err, "failed to click on chrome address bar")
			}
			if err := testing.Sleep(ctx, 1*time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep after clicking on the Chrome address bar")
			}
			if res, err := cvkc.CheckVirtualKeyboardIsPresent(ctx, &req); err != nil {
				return errors.Wrap(err, "failed to check whether virtual keyboard is present")
			} else if tabletMode != res.IsVirtualKeyboardPresent {
				return errors.Errorf(
					"found unexpected behavior, and got tabletmode: %t, VirtualKeyboardPresent: %t",
					tabletMode, res.IsVirtualKeyboardPresent)
			}
			return nil
		}, &checkVKPollOptions)
	}
	return nil
}
