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
		Func:         AssignToAllDesks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Assign apps to all desks",
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

func AssignToAllDesks(ctx context.Context, s *testing.State) {
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
	defer ash.CleanUpDesks(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
	}

	ac := uiauto.New(tconn)

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

	appsList := []apps.App{chromeApp, apps.Terminal, arcApp}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
		}
	}

	// Creat 4 desks.
	const numNewDesks = 4
	for i := 1; i <= numNewDesks; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %d: %v", i+1, err)
		}
	}

	// 5. Assign window to all desks.
	if err := assignWindowsToDesks(ctx, tconn, ac, true); err != nil {
		s.Fatal("Failed to assign window to all desks: ", err)
	}
	// 6. Verify window shows up on all desks.
	if err := verifyWindowsOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}

	// 7. Create two more desks and verify the windows also can show up on the new created desks.
	for i := 0; i <= 2; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %d: %v", i, err)
		}
	}
	if err := verifyWindowsOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}

	// 8. Close Desk 4.
	info, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get desk info: ", err)
	}
	if info.ActiveDeskIndex != 3 {
		if err := ash.ActivateDeskAtIndex(ctx, tconn, 3); err != nil {
			s.Fatal("Failed to activate Desk 4: ", err)
		}
	}
	if err := ash.RemoveActiveDesk(ctx, tconn); err != nil {
		s.Fatal("Failed to remove active Desk 4: ", err)
	}
	if err := verifyWindowsOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}

	// 9. Close the window on Desk 3.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find all windows: ", err)
	}
	if len(ws) != 3 {
		s.Fatalf("Unexpected number of windows found; want: 3, got: %d; %v", len(ws), err)
	}
	for i := 0; i < len(ws); i++ {
		if err := ws[i].CloseWindow(ctx, tconn); err != nil {
			s.Fatal("Failed to close the window: ", err)
		}
	}
	// Verify no windows on any Desk except Desk 3.
	if err := ash.ActivateDeskAtIndex(ctx, tconn, 0); err != nil {
		s.Fatal("Failed to activate Desk 1: ", err)
	}
	// TODO(b/246782864): Use a proper wait for the desk animation.
	// Make sure the desk animiation is finished.
	if err := ac.WithInterval(2*time.Second).WithTimeout(10*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
		s.Fatal("Failed to wait for desk animation finished: ", err)
	}
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find all windows: ", err)
	}
	if len(ws) != 0 {
		s.Fatalf("Unexpected number of windows found; want: 0, got: %d; %v", len(ws), err)
	}

	// 10. Re-open the 3 kinds of windows and assign them to all desks. Then re-assign them to a specific desk.
	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %v", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %v", app.Name, err)
		}
	}
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find all windows: ", err)
	}
	if len(ws) != 3 {
		s.Fatalf("Unexpected number of windows found; want: 3, got: %d; %v", len(ws), err)
	}

	// Assign window to all desks and verify it shows up on all desks.
	if err := assignWindowsToDesks(ctx, tconn, ac, true); err != nil {
		s.Fatal("Failed to assign windows to all desks: ", err)
	}
	if err := verifyWindowsOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}
	// Re-assign it to Desk 2 and verify it only shows up on Desk 2.
	if err := assignWindowsToDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to assign windows to Desk 2: ", err)
	}
	if err := verifyWindowsOnDesks(ctx, tconn, ac, true); err != nil {
		s.Fatal("Failed to verify the window only on Desk 2: ", err)
	}
}

// assignWindowsToDesks assigns windows to all desks or Desk 2 based on `toAllDesks`.
func assignWindowsToDesks(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, toAllDesks bool) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find all windows")
	}
	numWindows := 3
	if len(ws) != 3 {
		return errors.Wrapf(err, "unexpected number of windows found; want: %d, got: %d", numWindows, len(ws))
	}

	var moveTarget *nodewith.Finder
	// Assign window to all desks or Desk 2.
	if toAllDesks {
		moveTarget = nodewith.ClassName("MenuItemView").Name("All desks")
	} else {
		moveTarget = nodewith.ClassName("MenuItemView").Name("Desk 2")
	}

	for i := 0; i < numWindows; i++ {
		// Right click on the top of the window.
		rightClickPoint := coords.NewPoint(ws[i].BoundsInRoot.CenterPoint().X, ws[i].BoundsInRoot.Top+10)
		if err := mouse.Click(tconn, rightClickPoint, mouse.RightButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to right click the top of the window")
		}

		// Move mouse to the move window to desk menu item.
		moveWindowToDeskMenuItem := nodewith.ClassName("MenuItemView").Name("Move window to desk")
		if err := ac.MouseMoveTo(moveWindowToDeskMenuItem, 0)(ctx); err != nil {
			return errors.Wrap(err, "failed to move mouse to the move window to desk menu item")
		}

		if err := ac.DoDefault(moveTarget)(ctx); err != nil {
			return errors.Wrap(err, "failed to move the window to Desk 2")
		}

		if err := ash.WaitWindowFinishAnimating(ctx, tconn, ws[i].ID); err != nil {
			return errors.Wrap(err, "failed to wait window finish animating")
		}
	}

	return nil
}

// verifyWindowsOnDesks verifies windows showing up on all desks or only on Desk 2 based on `showOnDesk2`.
func verifyWindowsOnDesks(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, showOnDesk2 bool) error {
	info, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the desk info")
	}
	numDesks := info.NumDesks

	for i := 0; i < numDesks; i++ {
		if info.ActiveDeskIndex != i {
			if err := ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
				return errors.Wrapf(err, "failed to activate Desk %v", i+1)
			}
		}

		// TODO(b/246782864): Use a proper wait for the desk animation.
		// Make sure the desk animiation is finished.
		if err := ac.WithInterval(2*time.Second).WithTimeout(10*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for desk animation finished")
		}

		count, err := ash.CountVisibleWindows(ctx, tconn)
		if err != nil {
			return errors.Wrapf(err, "failed to find the window on Desk %v", i+1)
		}
		if showOnDesk2 {
			// Any desks except Desk 2 shouldn't own any windows.
			if count != 0 && i != 1 {
				return errors.Wrap(err, "unexpected found windows on desk except for Desk 2")
			}
			// Desk 2 should have 3 kinds of windows.
			if i == 1 && count != 3 {
				return errors.Wrapf(err, "failed to find windows on Desk 2; want: 3, got: %d", count)
			}
		} else {
			if count != 3 {
				return errors.Wrap(err, "failed to find 3 kinds of windows on all desks")
			}
		}
	}

	return nil
}
