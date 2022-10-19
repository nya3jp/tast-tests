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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui/terminalapp"
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
		Params: []testing.Param{
			{
				Name:    "browser",
				Val:     ash.WindowTypeBrowser,
				Fixture: "chromeLoggedIn",
			},
			{
				Name:    "crostini_app",
				Val:     ash.WindowTypeCrostini,
				Fixture: "crostiniBuster",
			},
			{
				Name: "chrome_app",
				Val:  ash.WindowTypeChromeApp,
			},
			{
				Name:    "arc_app",
				Val:     ash.WindowTypeArc,
				Fixture: "arcBooted",
			},
		},
	})
}

func AssignToAllDesks(ctx context.Context, s *testing.State) {
	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testAppType := s.Param().(ash.WindowType)
	var cr *chrome.Chrome
	switch testAppType {
	case ash.WindowTypeBrowser:
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
	case ash.WindowTypeCrostini:
		cr = s.FixtValue().(crostini.FixtureData).Chrome
	case ash.WindowTypeChromeApp:
		var err error
		cr, err = chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to start chrome: ", err)
		}
	case ash.WindowTypeArc:
		cr = s.FixtValue().(*arc.PreData).Chrome
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	var appIDToLaunch string

	// Install the paramaterized app type and get the appIDToLaunch
	switch testAppType {
	case ash.WindowTypeBrowser:
		browserApp, err := apps.PrimaryBrowser(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find browser app info: ", err)
		}
		appIDToLaunch = browserApp.ID
	// case ash.WindowTypeCrostini:
	case ash.WindowTypeChromeApp:
		chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
		if err != nil {
			s.Fatal("Could not find the Chrome app: ", err)
		}
		appIDToLaunch = chromeApp.ID
	case ash.WindowTypeArc:
		var err error
		const apk = "ArcInstallAppWithAppListSortedTest.apk"
		a := s.FixtValue().(*arc.PreData).ARC
		if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
			s.Fatal("Failed installing arc app: ", err)
		}

		const appName = "InstallAppWithAppListSortedMockApp"
		appIDToLaunch, err = ash.WaitForChromeAppByNameInstalled(ctx, tconn, appName, 1*time.Minute)
		if err != nil {
			s.Fatalf("Failed to wait until %s is installed: %v", appName, err)
		}
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

	// Creat 4 desks.
	const numDesks = 5
	for i := 2; i <= numDesks; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %d: %v", i, err)
		}
	}

	// Launch the paramaterized app type.
	if testAppType == ash.WindowTypeCrostini {
		_, err := terminalapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch terminal app after installing Crostini: ", err)
		}
	} else {
		if err = apps.Launch(ctx, tconn, appIDToLaunch); err != nil {
			s.Fatalf("Failed to launch %s: %v", testAppType, err)
		}
		if err = ash.WaitForApp(ctx, tconn, appIDToLaunch, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %v", testAppType, err)
		}
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find all windows: ", err)
	}
	if len(ws) != 1 {
		s.Fatalf("Unexpected number of windows found; want: 1, got: %d; %v", len(ws), err)
	}

	// Assign window to all desks.
	if err := assignWindowToDesks(ctx, tconn, ac, ws[0], true); err != nil {
		s.Fatal("Failed to assign window to all desks: ", err)
	}

	if err := verifyWindowOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}

	// Create two more desks.
	for i := 0; i <= 2; i++ {
		if err := ash.CreateNewDesk(ctx, tconn); err != nil {
			s.Fatalf("Failed to create the Desk %d: %v", i, err)
		}
	}

	if err := verifyWindowOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}

	// Close Desk 4.
	info, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get desk info: ", err)
	}
	if info.ActiveDeskIndex != 3 {
		if err := ash.ActivateDeskAtIndex(ctx, tconn, 3); err != nil {
			s.Fatal("Failed to active Desk 4: ", err)
		}
	}

	if err := verifyWindowOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}

	// Close the window.
	if err := ws[0].CloseWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to close the window: ", err)
	}
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find all windows: ", err)
	}
	if len(ws) != 0 {
		s.Fatalf("Unexpected number of windows found; want: 0, got: %d; %v", len(ws), err)
	}

	// Launch the paramaterized app type.
	if testAppType == ash.WindowTypeCrostini {
		_, err := terminalapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch terminal app after installing Crostini: ", err)
		}
	} else {
		if err = apps.Launch(ctx, tconn, appIDToLaunch); err != nil {
			s.Fatalf("Failed to launch %s: %v", testAppType, err)
		}
		if err = ash.WaitForApp(ctx, tconn, appIDToLaunch, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %v", testAppType, err)
		}
	}

	// Assign window to all desks and then re-assign it to Desk 2.
	if err := assignWindowToDesks(ctx, tconn, ac, ws[0], true); err != nil {
		s.Fatal("Failed to assign window to all desks: ", err)
	}
	if err := verifyWindowOnDesks(ctx, tconn, ac, false); err != nil {
		s.Fatal("Failed to verify the window on all desks: ", err)
	}
	if err := assignWindowToDesks(ctx, tconn, ac, ws[0], false); err != nil {
		s.Fatal("Failed to assign window to all desks: ", err)
	}
	if err := verifyWindowOnDesks(ctx, tconn, ac, true); err != nil {
		s.Fatal("Failed to verify the window only on Desk 2: ", err)
	}

}

func assignWindowToDesks(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, w *ash.Window, toAllDesks bool) error {
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

	var moveTarget *nodewith.Finder
	// Assign window to all desks or Desk 2.
	if toAllDesks {
		moveTarget = nodewith.ClassName("MenuItemView").Name("All desks")
	} else {
		moveTarget = nodewith.ClassName("MenuItemView").Name("Desk 2")
	}
	if err := ac.LeftClick(moveTarget)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the window to Desk 2")
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
		return errors.Wrap(err, "failed to wait window finish animating")
	}
	return nil
}

func verifyWindowOnDesks(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context, showOnDesk2 bool) error {
	info, err := ash.GetDesksInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the desk info")
	}
	numDesks := info.NumDesks

	for i := 0; i < numDesks; i++ {
		if info.ActiveDeskIndex != i {
			if err := ash.ActivateDeskAtIndex(ctx, tconn, i); err != nil {
				return errors.Wrapf(err, "failed to active Desk %v", i+1)
			}
		}
		count, err := ash.CountVisibleWindows(ctx, tconn)
		if err != nil {
			return errors.Wrapf(err, "failed to find the window on Desk %v", i+1)
		}
		if showOnDesk2 {
			if count != 0 && i != 1 {
				return errors.Wrap(err, "unexpected found window on desk except for Desk 2")
			}
		} else {
			if count != 1 {
				return errors.Wrap(err, "failed to find window on all desks")
			}
		}
	}

	return nil
}
