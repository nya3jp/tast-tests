// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shelf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotseatScalable,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the basic features of hotseat",
		Contacts: []string{
			"victor.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"tbarzic@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// HotseatScalable verifies the launcher icon, pinned apps and status menu should be displayed in clamshell mode.
func HotseatScalable(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	ui := uiauto.New(tconn)

	if err := ash.WaitForHotseatAnimatingToIdealState(ctx, tconn, ash.ShelfShownClamShell); err != nil {
		s.Fatal("Failed to find launcher button: ", err)
	}

	resetPinState, err := ash.ResetShelfPinState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the function to reset pin states: ", err)
	}
	defer resetPinState(cleanupCtx)

	app := apps.FilesSWA

	if err := ash.PinApps(ctx, tconn, []string{app.ID}); err != nil {
		s.Fatal("Failed to pin apps to the shelf: ", err)
	}

	ids, err := ash.GetPinnedAppIds(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get pinned app ids: ", err)
	}

	var appExist bool
	for _, id := range ids {
		if app.ID == id {
			appExist = true
			break
		}
	}
	if !appExist {
		s.Fatal("Failed to find app in pinned list")
	}

	if err := uiauto.Combine("wait launcher and status menu exist",
		ui.WaitUntilExists(nodewith.Name("Launcher").HasClass("ash/HomeButton").Role(role.Button)),
		ui.WaitUntilExists(nodewith.HasClass("UnifiedSystemTray").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to verify launcher or status menu: ", err)
	}
}
