// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MoveTabToAnotherWindowMenu,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Check if the move tab to another window menu is grouped by desks",
		Contacts: []string{
			"hongyulong@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func MoveTabToAnotherWindowMenu(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Open a browser window on Desk 1.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}
	if err := ash.WaitForApp(ctx, tconn, browserApp.ID, time.Minute); err != nil {
		s.Fatal("Browser did not appear in shelf after launch: ", err)
	}

	// Create 4 desks.
	const numNewDesks = 4
	for i := 1; i <= numNewDesks; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %v: %v", i+1, err)
		}
		if err := ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
			s.Fatalf("Failed to activate Desk %v: %v", i+1, err)
		}

		// Open a browser window.
		browserApp, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find browser app info: ", err)
		}
		if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
			s.Fatal("Failed to launch chrome: ", err)
		}
		if _, err := ash.WaitForAnyWindow(ctx, tconn, func(w *ash.Window) bool { return w.OnActiveDesk && w.IsVisible && !w.IsAnimating }); err != nil {
			s.Fatal("Failed to open and wait for browser window on active desk: ", err)
		}
	}
	ac := uiauto.New(tconn)
	if err := verifyTabGroupMenu(ctx, tconn, ac); err != nil {
		s.Fatal("Failed to verify the tab group menu: ", err)
	}
}

func verifyTabGroupMenu(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context) error {
	info, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the desk info")
	}
	numDesks := info.NumDesks

	// Activate Desk 1.
	if info.ActiveDeskIndex != 0 {
		if err := ash.ActivateDeskAtIndex(ctx, tconn, 0); err != nil {
			return errors.Wrap(err, "failed to activate Desk 1")
		}
	}

	// Right click on a tab and move mouse to move tab to another window item.
	tab := nodewith.ClassName("Tab").Name("New Tab")
	if err := ac.RightClick(tab)(ctx); err != nil {
		return errors.Wrap(err, "failed to right click the tab view")
	}

	moveTabToAnotherWindowItem := nodewith.ClassName("MenuItemView").Name("Move tab to another window")
	if err := ac.MouseMoveTo(moveTabToAnotherWindowItem, 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to the move tab to another window item")
	}

	// Verify the tab group menu.
	for i := 2; i <= numDesks; i++ {
		deskItem := nodewith.ClassName("MenuItemView").Name(fmt.Sprintf("Desk Desk %d has 1 browser windows open", i))
		if err := ac.WithTimeout(5 * time.Second).WaitUntilExists(deskItem)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find Desk %d item", i)
		}
		tabItem := nodewith.ClassName("MenuItemView").Name(fmt.Sprintf("New Tab belongs to desk Desk %d", i))
		if err := ac.WithTimeout(5 * time.Second).WaitUntilExists(tabItem)(ctx); err != nil {
			return errors.Wrapf(err, "failed to find the tab item of Desk %d", i)
		}
	}
	return nil
}
