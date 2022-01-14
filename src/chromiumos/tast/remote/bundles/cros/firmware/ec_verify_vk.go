// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParamsTablet struct {
	canDoTabletSwitch bool
	tabletModeOn      string
	tabletModeOff     string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECVerifyVK,
		Desc:         "Verify whether virtual keyboard window is present during change in tablet mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.CheckVirtualKeyboardService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Params: []testing.Param{{
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Convertible)),
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				tabletModeOn:      "tabletmode on",
				tabletModeOff:     "tabletmode reset",
			},
		}, {
			Name:              "detachable",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Detachable)),
			Val: &testParamsTablet{
				canDoTabletSwitch: true,
				tabletModeOn:      "basestate detach",
				tabletModeOff:     "basestate attach",
			},
		}, {
			Name:              "chromeslate",
			ExtraHardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromeslate)),
			Val: &testParamsTablet{
				canDoTabletSwitch: false,
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

	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	cvkc := pb.NewCheckVirtualKeyboardServiceClient(h.RPCClient.Conn)

	s.Log("Starting a new Chrome session and logging in as test user")
	if _, err := cvkc.NewChromeLoggedIn(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	s.Log("Opening a Chrome page")
	if _, err := cvkc.OpenChromePage(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to open chrome: ", err)
	}

	args := s.Param().(*testParamsTablet)

	// Chromeslates are already in tablet mode, and for this reason,
	// we could skip switching to tablet mode, and just verify that
	// virtual keyboard is present after a click on the address bar.
	if args.canDoTabletSwitch == false {
		if err := verifyVKIsPresent(ctx, h, cvkc, s, true, ""); err != nil {
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
			s.Logf("Tablet mode on: %t", dut.tabletMode)
			if err := verifyVKIsPresent(ctx, h, cvkc, s, dut.tabletMode, dut.tabletState); err != nil {
				s.Fatal("Failed to verify virtual keyboard status: ", err)
			}
		}
	}
}

func verifyVKIsPresent(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, s *testing.State, tabletMode bool, command string) error {
	// Run EC command to put DUT in clamshell/tablet mode.
	if command != "" {
		if err := h.Servo.RunECCommand(ctx, command); err != nil {
			return errors.Wrap(err, "failed to set DUT tablet mode state")
		}
	}
	// Wait for the command on switching to tablet mode to fully propagate,
	// before clicking on the address bar.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed in sleeping for one second before clicking on the address bar")
	}

	req := pb.CheckVirtualKeyboardRequest{
		IsDutTabletMode: tabletMode,
	}
	// Use polling here to wait till the UI tree has fully updated,
	// and check if virtual keyboard is present.
	s.Logf("Expecting virtual keyboard present: %t", tabletMode)
	return testing.Poll(ctx, func(c context.Context) error {
		s.Log("Clicking on the address bar of the Chrome page")
		if _, err := cvkc.ClickChromeAddressBar(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "failed to click chrome address bar")
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
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}
