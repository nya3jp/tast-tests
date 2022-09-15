// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type dutType int

const (
	convertible dutType = iota
	detachable
	chromeslate
)

type dutTestParams struct {
	canDoTabletSwitch bool
	formFactor        dutType
	tabletModeOn      string
	tabletModeOff     string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECVerifyVK,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify whether virtual keyboard window is present during change in tablet mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		// TODO(b/200305355): Add back to firmware_unstable when this test passes.
		Attr:         []string{"group:firmware", "firmware_detachable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.ScreenRecorderService", "tast.cros.browser.ChromeService", "tast.cros.ui.CheckVirtualKeyboardService", "tast.cros.firmware.UtilsService"},
		Fixture:      fixture.NormalMode,
		Timeout:      8 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC(), hwdep.TouchScreen()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val: dutTestParams{
				canDoTabletSwitch: true,
				formFactor:        convertible,
				tabletModeOn:      "tabletmode on",
				tabletModeOff:     "tabletmode off",
			},
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable)),
			Val: dutTestParams{
				canDoTabletSwitch: true,
				formFactor:        detachable,
				tabletModeOn:      "basestate detach",
				tabletModeOff:     "basestate attach",
			},
		}, {
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val: dutTestParams{
				canDoTabletSwitch: false,
				formFactor:        chromeslate,
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

	if err := h.RequireRPCUtils(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Temporary sleep would help prevent the streaming RPC call error.
	s.Log("Sleeping for a few seconds before starting a new Chrome")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep for a few seconds: ", err)
	}

	s.Log("Logging in as a guest user")
	chromeService := pb.NewChromeServiceClient(h.RPCClient.Conn)
	if _, err := chromeService.New(ctx, &pb.NewRequest{
		LoginMode: pb.LoginMode_LOGIN_MODE_GUEST_LOGIN,
	}); err != nil {
		s.Fatal("Failed to login: ", err)
	}
	defer chromeService.Close(ctx, &empty.Empty{})

	s.Log("Screen recorder started")
	filePath := filepath.Join(s.OutDir(), "ecVerifyVK.webm")
	startRequest := pb.StartRequest{
		FileName: filePath,
	}
	screenRecorder := pb.NewScreenRecorderServiceClient(h.RPCClient.Conn)

	if _, err := screenRecorder.Start(ctx, &startRequest); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	defer func(ctx context.Context) {
		res, err := screenRecorder.Stop(ctx, &empty.Empty{})
		if err != nil {
			s.Log("Unable to save the recording: ", err)
		} else {
			s.Logf("Screen recording saved to %s", res.FileName)
		}
	}(cleanupCtx)

	// Restore tablet mode settings so that DUT won't
	// be left in tablet mode at the end of test.
	args := s.Param().(dutTestParams)
	ecTool := firmware.NewECTool(s.DUT(), firmware.ECToolNameMain)
	var restoreECTabletMode bool
	defer func(ctx context.Context, restoreECTabletMode *bool) {
		if *restoreECTabletMode {
			s.Log("Restoring ec tablet mode setting at the end of test")
			if _, err := h.Servo.CheckAndRunTabletModeCommand(ctx, args.tabletModeOff); err != nil {
				s.Fatal("Unablet to reset EC tablet mode setting: ", err)
			}
		}
		if args.formFactor == convertible {
			// Set tablet mode angles to the default settings under ectool
			// at the end of test if they are not.
			tabletModeAngleInit, hysInit, err := ecTool.SaveTabletModeAngles(ctx)
			if err != nil {
				s.Fatal("Failed to read tablet mode angles: ", err)
			} else if tabletModeAngleInit != "180" || hysInit != "20" {
				s.Log("Restoring ectool tablet mode angles to the default settings")
				if err := ecTool.ForceTabletModeAngle(ctx, "180", "20"); err != nil {
					s.Fatal("Failed to restore tablet mode angles to the default settings: ", err)
				}
			}
		}
	}(cleanupCtx, &restoreECTabletMode)

	vkService := pb.NewCheckVirtualKeyboardServiceClient(h.RPCClient.Conn)
	for _, tc := range []struct {
		formFactor        dutType
		canDoTabletSwitch bool
		turnTabletModeOn  bool
		tabletModeCmd     string
	}{
		{args.formFactor, args.canDoTabletSwitch, true, args.tabletModeOn},
		{args.formFactor, args.canDoTabletSwitch, false, args.tabletModeOff},
	} {
		// Switch DUT to tablet mode, then back to clamshell mode, for convertibles and detachables.
		msg, err := switchDUTMode(ctx, h, tc.canDoTabletSwitch, tc.turnTabletModeOn, tc.tabletModeCmd, ecTool)
		if err != nil {
			s.Fatalf("Failed to run %s: %v", tc.tabletModeCmd, err)
		}
		const tabletModeEnabled = "tablet mode enabled"
		switch strings.Contains(msg, tabletModeEnabled) {
		case true:
			restoreECTabletMode = true
		default:
			restoreECTabletMode = false
		}
		// Short delay to ensure that the command on changing DUT's tablet mode state has fully propagated.
		s.Log("Sleeping for a few seconds")
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
		s.Log("Checking tablet mode status")
		dutInTabletMode, err := checkTabletMode(ctx, h, tc.turnTabletModeOn)
		if err != nil {
			s.Fatal("Failed to check DUT's tablet mode status: ", err)
		}
		s.Log("Using search bar to trigger virtual keyboard")
		if _, err := vkService.ClickSearchBar(ctx, &pb.CheckVirtualKeyboardRequest{
			IsDutTabletMode: dutInTabletMode,
		}); err != nil {
			s.Fatal("Failed to click Search Bar: ", err)
		}
		s.Log("Checking if virtual keyboard is present")
		if err := checkVKIsPresent(ctx, h, vkService, dutInTabletMode); err != nil {
			s.Fatal("Failed to check VK is present: ", err)
		}
		if tc.formFactor == chromeslate {
			// Because chromeslates do not support clamshell mode,
			// skip the clamshell mode test.
			break
		}
	}
}

func switchDUTMode(ctx context.Context, h *firmware.Helper, canDoTabletSwitch, turnTabletModeOn bool, tabletModeCmd string, ecTool *firmware.ECTool) (string, error) {
	if !canDoTabletSwitch {
		return "", nil
	}
	forceTabletModeAngle := func(ctx context.Context) error {
		if turnTabletModeOn {
			// Setting tabletModeAngle to 0s will force DUT into tablet mode.
			if err := ecTool.ForceTabletModeAngle(ctx, "0", "0"); err != nil {
				return errors.Wrap(err, "failed to force DUT into tablet mode")
			}
		} else {
			// Setting tabletModeAngle to 360 will force DUT into clamshell mode.
			if err := ecTool.ForceTabletModeAngle(ctx, "360", "0"); err != nil {
				return errors.Wrap(err, "failed to force DUT into clamshell mode")
			}
		}
		return nil
	}
	testing.ContextLogf(ctx, "Running EC command %s to change DUT's tablet mode state", tabletModeCmd)
	out, err := h.Servo.CheckAndRunTabletModeCommand(ctx, tabletModeCmd)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to set DUT tablet mode state, and got: %v. Attempting to set tablet_mode_angle with ectool instead", err)
		if err := forceTabletModeAngle(ctx); err != nil {
			return "", errors.Wrap(err, "failed to set DUT tablet mode state")
		}
		return "", nil
	}
	return out, nil
}

func checkTabletMode(ctx context.Context, h *firmware.Helper, turnTabletModeOn bool) (bool, error) {
	// Reuse the existing guest session created via ChromeService at the beginning of test.
	if _, err := h.RPCUtils.ReuseChrome(ctx, &empty.Empty{}); err != nil {
		return false, errors.Wrap(err, "failed to reuse Chrome session for the utils service")
	}
	res, err := h.RPCUtils.EvalTabletMode(ctx, &empty.Empty{})
	if err != nil {
		return false, errors.Wrap(err, "unable to evaluate whether ChromeOS is in tablet mode")
	} else if res.TabletModeEnabled != turnTabletModeOn {
		return false, errors.Errorf("expecting tablet mode on: %t, but got: %t", turnTabletModeOn, res.TabletModeEnabled)
	}
	testing.ContextLogf(ctx, "ChromeOS in tabletmode: %t", res.TabletModeEnabled)
	return res.TabletModeEnabled, nil
}

func checkVKIsPresent(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, tabletMode bool) error {
	req := pb.CheckVirtualKeyboardRequest{
		IsDutTabletMode: tabletMode,
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
}
