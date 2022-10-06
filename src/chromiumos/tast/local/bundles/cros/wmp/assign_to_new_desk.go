// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AssignToNewDesk,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Assign apps to a new desk",
		Contacts: []string{
			"hongyulong@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Val: browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func AssignToNewDesk(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Set up the browser.
	bt := s.Param().(browser.Type)
	cr, _, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, lacrosfixt.NewConfig(),
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	// Setup for launching ARC apps.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}
	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(cleanupCtx)

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}

	ac := uiauto.New(tconn)

	// Creat 5 desks.
	const numDesks = 5
	for i := 2; i <= numDesks; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %d: %v", i, err)
		}
		defer ash.CleanUpDesks(cleanupCtx, tconn)
	}

	// Active Desk 3.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 2); err != nil {
		s.Fatal("Failed to active Desk 3: ", err)
	}

	// browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	// if err != nil {
	// 	s.Fatal("Could not find browser app info: ", err)
	// }

	for _, app := range []apps.App{apps.PlayStore} {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to open %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
		if err := ac.WithInterval(2*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			s.Fatal("Failed to wait for the app launch event to be completed: ", err)
		}

		// if err := moveAndVerifyWindowOnDesk(ctx, tconn, ac); err != nil {
		// 	s.Fatal("Failed to test for %v", app.Name, err)
		// }
		faillog.DumpUITree(ctx, s.OutDir(), tconn)
	}
}

func moveAndVerifyWindowOnDesk(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find all windows")
	}
	if len(ws) != 1 {
		return errors.Wrapf(err, "unexpected number of desks found; want: 1, got: %d", len(ws))
	}

	if err := clickToMoveWindow(ctx, tconn, ac, ws[0].ID); err != nil {
		return errors.Wrap(err, "failed to click and move window")
	}

	if err := verifyWindowOnAssignedDesk(ctx, tconn, ac, ws[0]); err != nil {
		return errors.Wrap(err, "failed to verify window on the assigned desk")
	}
	return nil
}

// clickToMoveWindow clicks the window and assigns it from Desk 3 to Desk 2.
func clickToMoveWindow(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, windowID int) error {
	// Right click on the top of the window.
	// TODO: change to top center instead of TopContainerView
	topContainerView := nodewith.ClassName("TopContainerView")
	if err := ac.RightClick(topContainerView.Nth(0))(ctx); err != nil {
		return errors.Wrap(err, "failed to right click top container view")
	}

	// Move mouse to the move window to desk menu item.
	moveWindowToDeskMenuItem := nodewith.ClassName("MenuItemView").Name("Move window to desk")
	if err := ac.MouseMoveTo(moveWindowToDeskMenuItem, 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to the move window to desk menu item")
	}

	// Click the menu item to move window to Desk 2.
	moveToDesk2 := nodewith.ClassName("MenuItemView").Name("Desk 2")
	if err := ac.LeftClick(moveToDesk2)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the window to Desk 2")
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, windowID); err != nil {
		return errors.Wrap(err, "failed to wait window finish animating")
	}
	return nil
}

// verifyWindowOnAssignedDesk verifies window unassigned from the currect desk and moved to the assigned desk, and checks the
// mini views also update correctly.
func verifyWindowOnAssignedDesk(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, window *ash.Window) error {
	// Check no window on the current desk.
	if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.OnActiveDesk && w.ID == window.ID
	}); err != nil && err != ash.ErrWindowNotFound {
		return errors.Wrap(err, "failed to found window on Desk 3")
	}

	// Active Desk 2.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		return errors.Wrap(err, "failed to active Desk 2")
	}

	// Find window on the assigned desk, which is the current desk now.
	if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
		return w.OnActiveDesk && w.ID == window.ID
	}); err != nil {
		return errors.Wrap(err, "failed to found window on Desk 2")
	}

	// Close window on the assigned desk.
	if err := window.CloseWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close the window")
	}

	// Active Desk 3.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to active Desk 3")
	}

	return nil
}
