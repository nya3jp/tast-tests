// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: VirtualDesksChromeApp,
		Desc: "Checks that virtual desks works correctly when creating apps from tabs",
		Contacts: []string{
			"shidi@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func VirtualDesksChromeApp(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	ac := uiauto.New(tconn)
	kb, err := input.Keyboard(ctx)

	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	pc := pointer.NewMouse(tconn)
	defer pc.Close()

	// Install Youtube as a PWA App.
	// TODO(crbug/1261204): Need autotest API to simulate PWA apps.
	youtubeAppID, err := apps.InstallPWAForURL(ctx, cr, "https://www.youtube.com/", 15*time.Second)
	if err != nil {
		s.Fatal("Failed to install PWA for URL: ", err)
	}

	if err := ash.WaitForChromeAppInstalled(ctx, tconn, youtubeAppID, 15*time.Second); err != nil {
		s.Fatal("Failed to wait for PWA to be installed: ", err)
	}
	// Pin Youtube app to the shelf.
	if err := ash.PinApps(ctx, tconn, []string{youtubeAppID}); err != nil {
		s.Fatal("Pin apps to the shelf from the launcher: ", err)
	}

	// Opens Chrome.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatalf("Failed to open %s: %v", apps.Chrome.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %s", apps.Chrome.Name, err)
	}

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	// Creates new desk and enters it.
	addDeskButton := nodewith.ClassName("ZeroStateIconButton")
	newDeskNameView := nodewith.ClassName("DeskNameView").Name("Desk 2")
	newDeskMiniView := nodewith.ClassName("DeskMiniView").Nth(1)
	newDeskName := "Desk 2"
	if err := uiauto.Combine(
		"create a new desk",
		ac.LeftClick(addDeskButton),
		// The focus on the new desk should be on the desk name field.
		ac.WaitUntilExists(newDeskNameView.Focused()),
		kb.TypeAction(newDeskName),
		kb.AccelAction("Enter"),
		ac.LeftClick(newDeskMiniView),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new desk: ", err)
	}

	// Verifies that there are 2 desks.
	deskMiniViewsInfo, err := ac.NodesInfo(ctx, nodewith.ClassName("DeskMiniView"))
	if err != nil {
		s.Fatal("Failed to find desks: ", err)
	}
	if len(deskMiniViewsInfo) != 2 {
		s.Fatalf("Expected %v desks, but got %v instead", 2, len(deskMiniViewsInfo))
	}

	// Open a Chrome browser.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatalf("Failed to open %s: %v", apps.Chrome.Name, err)
	}
	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatalf("%s did not appear in shelf after launch: %s", apps.Chrome.Name, err)
	}

	// Checks that browser window is created in current desk,
	// even if there are other browser windows on other inactive desks.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if (w.Title == "Chrome - New Tab") && w.OnActiveDesk == false {

		}
		return nil
	}); err != nil {
		s.Fatal("Failed to verify the desk of the app: ", err)
	}

	youtubeBtn := nodewith.ClassName("ash/ShelfAppButton").Name("YouTube")

	if err := uiauto.Combine(
		"click YouTube shelf button",
		ac.LeftClick(youtubeBtn),
		ac.WaitForLocation(nodewith.ClassName("WebContentsViewAura").Name("YouTube")),
	)(ctx); err != nil {
		s.Fatal("Failed to click the Youtube button: ", err)
	}

	// Check no new window are created even after clicked Youtube button from desk 2.
	// TODO(crbug/1261206): Need autotest api to check which desk is the current active desk.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check no new window is created: ", err)
	}
	// 3 windows are created with previous procedure, ensure no new window is created.
	if len(ws) != 3 {
		s.Fatalf("Unexpected number of windows found; wanted %v, got %v", 3, len(ws))
	}

	// Enters overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set overview mode: ", err)
	}

	contextMenu := nodewith.ClassName("MenuHostRootView")
	newWindowBtn := nodewith.Name("New window").ClassName("MenuItemView")
	// Click new tab on the Youtube app from the shelf.
	if err := uiauto.Combine(
		"click new tab on youtube app",
		// Switch back to desk 2 first.
		ac.LeftClick(newDeskMiniView),
		ac.RightClick(youtubeBtn),
		ac.WaitUntilExists(contextMenu),
		ac.LeftClick(newWindowBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new youtube window: ", err)
	}

	// Checks that the new youtube window is on desk 2 instead of desk 1.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		if (w.Title == "Youtube - Youtube") && w.OnActiveDesk == false {
			return errors.New("youtube app should be in the active desk")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to verify the desk of the app: ", err)
	}
}
