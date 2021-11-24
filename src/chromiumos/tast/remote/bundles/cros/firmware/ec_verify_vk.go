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

func init() {
	testing.AddTest(&testing.Test{
		Func:         ECVerifyVK,
		Desc:         "Verify whether virtual keyboard window is present during change in tablet mode",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.ui.CheckVirtualKeyboardService"},
		Fixture:      fixture.NormalMode,
		HardwareDeps: hwdep.D(
			hwdep.ChromeEC(),
			hwdep.FormFactor(hwdep.Convertible, hwdep.Chromeslate, hwdep.Detachable),
		),
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

	for _, dut := range []struct {
		tabletMode bool
	}{
		{true},
		{false},
	} {
		s.Logf("Tablet mode on: %t", dut.tabletMode)
		if err := verifyVKIsPresent(ctx, h, cvkc, s, dut.tabletMode); err != nil {
			s.Fatal("Failed to verify virtual keyboard status: ", err)
		}
	}
}

func verifyVKIsPresent(ctx context.Context, h *firmware.Helper, cvkc pb.CheckVirtualKeyboardServiceClient, s *testing.State, tabletMode bool) error {
	// Run EC command to put DUT in clamshell/tablet mode.
	if tabletMode {
		if err := h.Servo.RunECCommand(ctx, "tabletmode on"); err != nil {
			return errors.Wrap(err, "failed to set DUT into tablet mode")
		}
	} else {
		if err := h.Servo.RunECCommand(ctx, "tabletmode reset"); err != nil {
			return errors.Wrap(err, "failed to restore DUT to the original tabletmode setting")
		}
	}

	s.Log("Clicking on the address bar of the Chrome page")
	if _, err := cvkc.ClickChromeAddressBar(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to click chrome address bar")
	}

	req := pb.CheckVirtualKeyboardRequest{
		IsDutTabletMode: tabletMode,
	}

	// Use polling here to wait till the UI tree has fully updated,
	// and check if virtual keyboard is present.
	s.Log("Checking if virtual keyboard is present")
	return testing.Poll(ctx, func(c context.Context) error {
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
	}, &testing.PollOptions{Timeout: 5 * time.Second, Interval: time.Second})
}
