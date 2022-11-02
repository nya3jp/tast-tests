// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/remote/bundles/cros/oobe/servoutil"
	crui "chromiumos/tast/remote/cros/ui"
	oobeutil "chromiumos/tast/remote/cros/ui/oobe"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidScreenUsbKeyboardOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a USB keyboard is connected to in OOBE",
		Contacts: []string{
			"tjohnsonkanu@google.com",
			"cros-connectivity@google.com",
		},
		VarDeps:      []string{"servo"},
		Attr:         []string{"group:mainline", "informational", "group:bluetooth", "bluetooth_btpeers_1"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps: []string{
			"tast.cros.ui.AutomationService",
			"tast.cros.ui.ChromeUIService",
			"tast.cros.bluetooth.BTTestService",
			"tast.cros.inputs.KeyboardService",
		},
		Fixture:      "chromeOobeWith1BTPeer",
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
	})
}

// HidScreenUsbKeyboardOnly tests that a single USB keyboard is connected to during OOBE.
func HidScreenUsbKeyboardOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	uiautoSvc := ui.NewAutomationServiceClient(fv.DUTRPCClient.Conn)
	crUISvc := ui.NewChromeUIServiceClient(fv.DUTRPCClient.Conn)
	keyboardSvc := inputs.NewKeyboardServiceClient(fv.DUTRPCClient.Conn)

	defer func() {
		if !s.HasError() {
			return
		}
		if _, err := crUISvc.DumpUITree(cleanupCtx, &emptypb.Empty{}); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to dump UI tree: ", err)
		}
	}()

	servoutil.TurnOffServoKeyboard(ctx, s)

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err == nil {
		s.Fatal("Continue button should be disabled: ", err)
	}

	// Connect USB keyboard and make sure continue button is enabled.
	if _, err := keyboardSvc.NewKeyboard(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to emulate keyboard: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.FoundUSBKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Disconnect keyboard and make sure continue button is disabled.
	if _, err := keyboardSvc.CloseKeyboard(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to disconnect keyboard: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.SearchingForKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err == nil {
		s.Fatal("Continue button should be disabled: ", err)
	}

	// Connect keyboard
	if _, err := keyboardSvc.NewKeyboard(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to emulate keyboard: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.FoundUSBKeyboardNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	if err := crui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err != nil {
		s.Fatal("Failed to find node: ", err)
	}

	// Navigate to welcome screen.
	if _, err := uiautoSvc.LeftClick(
		ctx, &ui.LeftClickRequest{Finder: oobeutil.ContinueButtonFinder}); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}

	if _, err := crUISvc.WaitForWelcomeScreen(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to enter welcome page")
	}

	// Cleanup keyboard resources.
	if _, err := keyboardSvc.CloseKeyboard(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to disconnect keyboard: ", err)
	}
}
