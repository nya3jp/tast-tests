// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/oobe"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HidPreconnectedTouchscreenOnly,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that OOBE HID Detection screen is skipped on non-applicable devices",
		Contacts: []string{
			"andrewdear@google.com",
			"cros-connectivity@google.com",
		},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		//HardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
		Fixture: "chromeOobeHidDetection",
		Timeout: time.Second * 15,
	})
}

// HidPreconnectedTouchscreenOnly checks that the touchscreen is enabled on a ChromeBase.
func HidPreconnectedTouchscreenOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*oobe.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	crUISvc := ui.NewChromeUIServiceClient(fv.DUTRPCClient.Conn)
	if _, err := crUISvc.WaitForWelcomeScreen(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to enter welcome page")
	}

	// rpcClient, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	// if err != nil {
	// 	s.Fatal("Failed to create RPC client: ", err)
	// }

	// cr := s.FixtValue().(*oobe.ChromeOobeHidDetection).Chrome

	// oobeConn, err := cr.WaitForOOBEConnection(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create OOBE connection: ", err)
	// }
	// defer oobeConn.Close()

	// if err := oobe.IsHidDetectionScreenVisible(ctx, oobeConn); err != nil {
	// 	s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	// }

	// if err := oobe.IsHidDetectionTouchscreenDetected(ctx, oobeConn); err != nil {
	// 	s.Fatal("Failed to find the text indicating that a pointer is connected: ", err)
	// }

	// if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
	// 	s.Fatal("Failed to detect an enabled continue button: ", err)
	// }

	// tconn, err := cr.SigninProfileTestAPIConn(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create the signin profile test API connection: ", err)
	// }

	// if err := oobe.IsHidDetectionKeyboardNotDetected(ctx, oobeConn, tconn); err != nil {
	// 	s.Fatal("Failed to detect that no keyboard was detected: ", err)
	// }

}
