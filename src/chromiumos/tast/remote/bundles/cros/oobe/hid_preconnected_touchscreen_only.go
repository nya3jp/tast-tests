// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/ctxutil"
	fixt "chromiumos/tast/remote/oobe"
	oobeService "chromiumos/tast/services/cros/chrome/uiauto/oobe"
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
		ServiceDeps: []string{
			"tast.cros.chrome.uiauto.oobe.OobeService",
		},
		//HardwareDeps: hwdep.D(hwdep.SkipOnFormFactor(hwdep.Chromebase, hwdep.Chromebox, hwdep.Chromebit)),
		Fixture: "chromeOobeHidDetection",
	})
}

// HidPreconnectedTouchscreenOnly checks that the touchscreen is enabled on a ChromeBase.
func HidPreconnectedTouchscreenOnly(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*fixt.FixtValue)

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(cleanupCtx, 5*time.Second)
	defer cancel()

	oobeSvc := oobeService.NewOobeServiceClient(fv.DUTRPCClient.Conn)

	if _, err := oobeSvc.HidPreconnectedTouchscreenOnly(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
}
