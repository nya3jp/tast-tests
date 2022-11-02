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
	"chromiumos/tast/remote/cros/ui"
	oobeutil "chromiumos/tast/remote/cros/ui/oobe"
	"chromiumos/tast/services/cros/inputs"
	uiService "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidScreenUsbMouseOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that a USB mouse is connected to in OOBE",
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
			"tast.cros.inputs.MouseService",
		},
		Fixture:      "chromeOobeWith1BTPeer",
		HardwareDeps: hwdep.D(hwdep.FormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
	})
}

// HidScreenUsbMouseOnly tests that a single USB mouse is connected to during OOBE.
func HidScreenUsbMouseOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	uiautoSvc := uiService.NewAutomationServiceClient(fv.DUTRPCClient.Conn)
	crUISvc := uiService.NewChromeUIServiceClient(fv.DUTRPCClient.Conn)
	mouseSvc := inputs.NewMouseServiceClient(fv.DUTRPCClient.Conn)

	defer func() {
		if !s.HasError() {
			return
		}
		if _, err := crUISvc.DumpUITree(cleanupCtx, &emptypb.Empty{}); err != nil {
			testing.ContextLog(cleanupCtx, "Failed to dump UI tree: ", err)
		}
	}()

	servoutil.TurnOffServoKeyboard(ctx, s)

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err == nil {
		s.Fatal("Expected the 'Continue' button to be disabled: ", err)
	}

	// Connect USB mouse and make sure continue button is enabled.
	if _, err := mouseSvc.NewMouse(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to emulate mouse: ", err)
	}

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.FoundUSBMouseNodeName); err != nil {
		s.Fatal("Expected a mouse to be found: ", err)
	}

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err != nil {
		s.Fatal("Expected the 'Continue' button to be enabled: ", err)
	}

	// Disconnect mouse and make sure continue button is disabled.
	if _, err := mouseSvc.CloseMouse(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to disconnect mouse: ", err)
	}

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.SearchingForMouseNodeName); err != nil {
		s.Fatal("Expected to be searching for a mouse: ", err)
	}

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err == nil {
		s.Fatal("Expected the 'Continue' button to be disabled: ", err)
	}

	// Connect mouse device and make sure continue button is enabled.
	if _, err := mouseSvc.NewMouse(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to emulate mouse: ", err)
	}

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.FoundUSBMouseNodeName); err != nil {
		s.Fatal("Expected a mouse to be found: ", err)
	}

	if err := ui.CheckNodeWithNameExists(ctx, uiautoSvc, oobeutil.ContinueButtonEnabledNodeName); err != nil {
		s.Fatal("Expected the 'Continue' button to be enabled: ", err)
	}

	// Navigate to welcome screen.
	if _, err := uiautoSvc.LeftClick(
		ctx, &uiService.LeftClickRequest{Finder: oobeutil.ContinueButtonFinder}); err != nil {
		s.Fatal("Failed to click continue button: ", err)
	}

	if _, err := crUISvc.WaitForWelcomeScreen(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to enter welcome page")
	}

	// Cleanup mouse resources.
	if _, err := mouseSvc.CloseMouse(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to disconnect mouse: ", err)
	}
}
