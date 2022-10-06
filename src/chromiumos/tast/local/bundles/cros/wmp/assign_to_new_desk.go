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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
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
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func AssignToNewDesk(ctx context.Context, s *testing.State) {
	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	ac := uiauto.New(tconn)

	// Creat 4 desks.
	const numDesks = 5
	for i := 2; i <= numDesks; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %d: %v", i, err)
		}
		defer ash.CleanUpDesks(cleanupCtx, tconn)
	}

	// Activate Desk 3.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 2); err != nil {
		s.Fatal("Failed to active Desk 3: ", err)
	}

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	// Install an arc app.
	a := s.FixtValue().(cuj.FixtureData).ARC

	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}
	const apk = "ArcInstallAppWithAppListSortedTest.apk"
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed installing arc app: ", err)
	}
	const appName = "InstallAppWithAppListSortedMockApp"
	arcAppID, err := ash.WaitForChromeAppByNameInstalled(ctx, tconn, appName, 1*time.Minute)
	if err != nil {
		s.Fatalf("Failed to wait until %s is installed: %v", appName, err)
	}
	arcApp := apps.App{ID: arcAppID, Name: appName}

	appsList := []apps.App{apps.Terminal, chromeApp, arcApp}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
		}
	}

	if err := moveAndVerifyWindowsOnDesk2(ctx, tconn, ac); err != nil {
		s.Fatal("Failed to test for ", apps.FilesSWA.Name, err)
	}
}

// moveAndVerifyWindowsOnDesk2 moves windows to Desk 2 from Desk 3, and verifies windows showing up on Desk 2.
func moveAndVerifyWindowsOnDesk2(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find all windows")
	}
	numWindows := 3

	if len(ws) != numWindows {
		return errors.Wrapf(err, "unexpected number of windows found; want: %d, got: %d", numWindows, len(ws))
	}

	for i := 0; i < numWindows; i++ {
		if err := clickToMoveWindow(ctx, tconn, ac, ws[i]); err != nil {
			return errors.Wrapf(err, "failed to click and move %v", ws[i].Name)
		}
	}

	// Active Desk 2.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 1); err != nil {
		return errors.Wrap(err, "failed to active Desk 2")
	}

	// TODO(b/246782864): Use a proper wait for the desk animation.
	// Make sure the desk animiation is finished.
	if err := ac.WithInterval(2*time.Second).WithTimeout(10*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait desk animation finished")
	}

	for _, window := range ws {
		if _, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return window.ID == w.ID && w.OnActiveDesk
		}); err != nil {
			return errors.Wrapf(err, "failed to find %v on Desk 2", window.Name)
		}
	}
	return nil
}

// clickToMoveWindow clicks the window and assigns it from Desk 3 to Desk 2.
func clickToMoveWindow(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, w *ash.Window) error {
	// Right click on the top of the window.
	rightClickPoint := coords.NewPoint(w.BoundsInRoot.CenterPoint().X, w.BoundsInRoot.Top+10)
	if err := mouse.Click(tconn, rightClickPoint, mouse.RightButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to right click the top of the window")
	}

	// Move mouse to the move window to desk menu item.
	moveWindowToDeskMenuItem := nodewith.ClassName("MenuItemView").Name("Move window to desk")
	if err := ac.MouseMoveTo(moveWindowToDeskMenuItem, 0)(ctx); err != nil {
		return errors.Wrap(err, "failed to move mouse to the move window to desk menu item")
	}

	// Click the menu item to move window to Desk 2.
	moveToDesk2 := nodewith.ClassName("MenuItemView").Name("Desk 2")
	if err := ac.DoDefault(moveToDesk2)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the window to Desk 2")
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
		return errors.Wrap(err, "failed to wait window finish animating")
	}
	return nil
}
