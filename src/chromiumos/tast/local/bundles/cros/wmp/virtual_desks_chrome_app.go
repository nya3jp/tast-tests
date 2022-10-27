// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualDesksChromeApp,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that virtual desks works correctly when creating apps from tabs",
		Contacts: []string{
			"shidi@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Data:         []string{"web_app_install_force_list_index.html", "web_app_install_force_list_manifest.json", "web_app_install_force_list_service-worker.js", "web_app_install_force_list_icon-192x192.png", "web_app_install_force_list_icon-512x512.png"},
	})
}

func VirtualDesksChromeApp(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	_, name, cleanUp, err := policyutil.InstallPwaAppByPolicy(ctx, tconn, cr, fdms, s.DataFileSystem())
	if err != nil {
		s.Fatal("Failed to install PWA: ", err)
	}

	defer cleanUp(ctx)

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

	// Wait until the PWA is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := launcher.SearchAndLaunch(tconn, kb, name)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s", name)
		}

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
		}

		for _, window := range windows {
			if window.Title == name {
				return nil
			}
		}
		return errors.New("failed to find a window with the PWA")
	}, nil); err != nil {
		s.Error("PWA wasn't installed: ", err)
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
	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)

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

	// Exit overview mode.
	if err := ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("Failed to exit overview mode: ", err)
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

	TestPWABtn := nodewith.ClassName("ash/ShelfAppButton").Name("Test PWA")

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the window list: ", err)
	}
	// 2 chrome windows and 1 PWA window are created with previous procedure.
	if len(ws) != 3 {
		s.Fatalf("Unexpected number of windows found; wanted %v, got %v", 3, len(ws))
	}
	// Click Test PWA shelf button, this will bring back to first desk.
	if err := uiauto.Combine(
		"click Test PWA shelf button",
		ac.LeftClick(TestPWABtn),
		ac.WaitForLocation(nodewith.ClassName("WebContentsViewAura").Name("Test PWA")),
	)(ctx); err != nil {
		s.Fatal("Failed to click the Test PWA button: ", err)
	}
	// Check which desk is the current active desk.
	info, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the desk info: ", err)
	}
	activeDesk := info.ActiveDeskIndex
	// Compare the actual active desk to the expected active desk.
	if activeDesk != 0 {
		s.Fatalf("Unexpected active desk: desk %d is active, expected desk 0 to be active", activeDesk)
	}
	currWs, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the window list: ", err)
	}
	// Ensure no new window is created.
	if len(currWs) != len(ws) {
		s.Fatalf("Unexpected number of windows found; wanted %v, got %v", len(ws), len(currWs))
	}

	contextMenu := nodewith.ClassName("MenuHostRootView")
	newWindowBtn := nodewith.Name("New window").ClassName("MenuItemView")
	// Click new tab on the Test PWA app from the shelf.
	if err := uiauto.Combine(
		"click new tab on Test PWA app",
		// Switch back to desk 2 first.
		kb.AccelAction("Search+]"),
		// This will wait until the container for desk 2 has become visible.
		ac.WaitForLocation(nodewith.ClassName("Desk_Container_B").State("invisible", false)),
		ac.RightClick(TestPWABtn),
		ac.WaitUntilExists(contextMenu),
		ac.LeftClick(newWindowBtn),
	)(ctx); err != nil {
		s.Fatal("Failed to create a new Test PWA window: ", err)
	}

	// Checks that the new Test PWA window is on desk 2 instead of desk 1.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		const name = "Test PWA - Test PWA"
		if (w.Title == name) && w.OnActiveDesk == false {
			return errors.New("Test PWA app should be in the active desk")
		}
		return nil
	}); err != nil {
		s.Error("Failed to verify the desk of the app: ", err)
	}
}
