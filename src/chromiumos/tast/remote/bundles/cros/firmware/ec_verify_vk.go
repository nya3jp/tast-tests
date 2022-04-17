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

// TODO: Find a home for browserType that could be referenced by both remote and local tests.
type browserType string

const (
	typeAsh    browserType = "ash"
	typeLacros browserType = "lacros"
)

type testParamsTablet struct {
	canDoTabletSwitch bool
	formFactor        string
	tabletModeOn      string
	tabletModeOff     string
	browserType       browserType
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECVerifyVK,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verify whether virtual keyboard window is present during change in tablet mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.CheckVirtualKeyboardService", "tast.cros.firmware.UtilsService", "tast.cros.ui.ChromeUIService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.TouchScreen()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				formFactor:        "convertible",
				tabletModeOn:      "tabletmode on",
				tabletModeOff:     "tabletmode off",
				browserType:       typeAsh,
			},
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable)),
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				formFactor:        "detachable",
				tabletModeOn:      "basestate detach",
				tabletModeOff:     "basestate attach",
				browserType:       typeAsh,
			},
		}, {
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val: &testParamsTablet{
				canDoTabletSwitch: false,
				formFactor:        "chromeslate",
				browserType:       typeAsh,
			},
		}, {
			Name:              "lacros",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			ExtraSoftwareDeps: []string{"lacros"},
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				formFactor:        "convertible",
				tabletModeOn:      "tabletmode on",
				tabletModeOff:     "tabletmode off",
				browserType:       typeLacros,
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
	args := s.Param().(*testParamsTablet)
	bt := args.browserType
	s.Log("Starting a new Chrome session and logging in as test user")
	if _, err := cvkc.NewChromeLoggedIn(ctx, newBrowserRequest(bt)); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	defer cvkc.CloseChrome(ctx, &empty.Empty{})

	// Open a new tab page.
	// Note that this could be skipped for lacros-chrome that has already opened the one when initializing the browser in NewChromeLoggedIn.
	if bt != typeLacros {
		s.Log("Opening a Chrome page for ash-chrome")
		if _, err := cvkc.OpenChromePage(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to open chrome: ", err)
		}
	}

	// Chromeslates are already in tablet mode, and for this reason,
	// we could skip switching to tablet mode, and just verify that
	// virtual keyboard is present after a click on the address bar.
	if args.canDoTabletSwitch == false {
		if err := verifyVKIsPresent(ctx, h, cvkc, s, true, "", args.formFactor); err != nil {
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
			if err := verifyVKIsPresent(ctx, h, cvkc, s, dut.tabletMode, dut.tabletState, args.formFactor); err != nil {
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

func verifyVKIsPresent(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, s *testing.State, tabletMode bool, command, dutFormFactor string) error {
	// Run EC command to put DUT in clamshell/tablet mode.
	if command != "" {
		if err := checkAndSetTabletMode(ctx, h, s, command); err != nil {
			if dutFormFactor == "convertible" {
				testing.ContextLogf(ctx, "Failed to set DUT tablet mode state, and got: %v. Attempting to set tablet_mode_angle with ectool instead", err)
				cmd := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
				// Save initial tablet mode angle settings to restore at the end of verifyVKIsPresent.
				tabletModeAngleInit, hysInit, err := cmd.SaveTabletModeAngles(ctx)
				if err != nil {
					return errors.Wrap(err, "failed to save initial tablet mode angles")
				}
				defer func() error {
					testing.ContextLogf(ctx, "Restoring DUT's tablet mode angles to the original settings: lid_angle=%s, hys=%s", tabletModeAngleInit, hysInit)
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

	// Create a Chrome instance for the utilsService by reusing one that's
	// already been created above under cvkc. The utilsService is required
	// by EvalTabletMode in checking tablet mode status.
	// Do not close the reused ash-chrome by calling utilsService.CloseChrome
	// since CheckVirtualKeyboardService manages its lifecycle.
	utilsService := fwpb.NewUtilsServiceClient(h.RPCClient.Conn)
	if _, err := utilsService.ReuseChrome(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to reuse Chrome session for the utils service")
	}

	// Log tablet mode status from the ChromeOS perspective.
	res, err := utilsService.EvalTabletMode(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "unable to evaluate whether ChromeOS is in tablet mode")
	}
	testing.ContextLogf(ctx, "ChromeOS in tabletmode: %t", res.TabletModeEnabled)

	// Use polling here to wait till the UI tree has fully updated,
	// and check if virtual keyboard is present. Start by sending
	// a tap on the touch screen, and if this fails in triggering the
	// vk, retry a few more times with a left-click.
	testing.ContextLogf(ctx, "Expecting virtual keyboard present: %t", tabletMode)
	if err := triggerAndCheckVK(ctx, h, cvkc, tabletMode, "byTouchScreen"); err != nil {
		testing.ContextLogf(ctx, "Checking virtual keyboard in tablet mode failed: %v. Retry a few more times with a left click", err)
		testing.ContextLog(ctx, "Restarting UI and logging in again")
		if err := refreshChromeAndLogin(ctx, h, cvkc, s.Param().(*testParamsTablet).browserType); err != nil {
			return err
		}
		testing.ContextLog(ctx, "Opening a new Chrome page")
		if _, err := cvkc.OpenChromePage(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to open a new chrome page")
		}
		// Left click on the new chrome page till vk appears.
		if err := triggerAndCheckVK(ctx, h, cvkc, tabletMode, "byLeftClick"); err != nil {
			return err
		}
		return nil
	}
	return nil
}

func triggerAndCheckVK(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, tabletMode bool, option string) error {
	req := pb.CheckVirtualKeyboardRequest{
		IsDutTabletMode: tabletMode,
	}
	// Check if the virtual keyboard is present first for the tablet mode.
	// Note that lacros-chrome brings it up automatically when a new tab is open, while ash-chrome doesn't.
	res, err := cvkc.CheckVirtualKeyboardIsPresent(ctx, &req)
	if err == nil && tabletMode == res.IsVirtualKeyboardPresent {
		return nil
	}

	// If the VK is not present, try to bring it up by either touching or clicking the address bar.
	return testing.Poll(ctx, func(ctx context.Context) error {
		switch option {
		case "byTouchScreen":
			testing.ContextLog(ctx, "Sending a touch on the address bar of a Chrome page")
			if _, err := cvkc.TouchChromeAddressBar(ctx, &empty.Empty{}); err != nil {
				return errors.Wrap(err, "failed to touch on chrome address bar")
			}
		case "byLeftClick":
			testing.ContextLog(ctx, "Clicking on the address bar of a Chrome page")
			if _, err := cvkc.ClickChromeAddressBar(ctx, &empty.Empty{}); err != nil {
				return errors.Wrap(err, "failed to click on chrome address bar")
			}
		}
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return errors.Wrap(err, "failed to sleep after clicking on the Chrome address bar")
		}
		res, err := cvkc.CheckVirtualKeyboardIsPresent(ctx, &req)
		if err != nil {
			return errors.Wrap(err, "failed to check whether virtual keyboard is present")
		} else if tabletMode != res.IsVirtualKeyboardPresent {
			return errors.Errorf(
				"found unexpected behavior, and got tabletmode: %t, VirtualKeyboardPresent: %t",
				tabletMode, res.IsVirtualKeyboardPresent)
		}
		return nil
	}, &testing.PollOptions{Timeout: 1 * time.Minute, Interval: 3 * time.Second})
}

func refreshChromeAndLogin(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, bt browserType) error {
	// Before restarting UI, close the chrome instance that was previously initiated.
	if _, err := cvkc.CloseChrome(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to close chrome")
	}
	// Restart UI, which would create a new Chrome session at login.
	serviceClient := pb.NewChromeUIServiceClient(h.RPCClient.Conn)
	if _, err := serviceClient.EnsureLoginScreen(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to restart ui")
	}
	// Delay for a few seconds to ensure that a new Chrome session has fully settled down.
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	// Log in again.
	if _, err := cvkc.NewChromeLoggedIn(ctx, newBrowserRequest(bt)); err != nil {
		return errors.Wrap(err, "failed to log in")
	}
	return nil
}

func newBrowserRequest(bt browserType) *pb.NewBrowserRequest {
	if bt == typeLacros {
		return &pb.NewBrowserRequest{BrowserType: pb.NewBrowserRequest_LACROS}
	}
	return &pb.NewBrowserRequest{BrowserType: pb.NewBrowserRequest_ASH}
}
