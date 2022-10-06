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

func AssignToNewDesk(ctx context.Context, s *testing.State) {
	var cr *chrome.Chrome

	testAppType := s.Param().(ash.WindowType)

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

		appName := "InstallAppWithAppListSortedMockApp"
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

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open: ", err)
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

	if err := moveAndVerifyWindowOnDesk(ctx, tconn, ac); err != nil {
		s.Fatal("Failed to test for ", apps.FilesSWA.Name, err)
	}
}

func moveAndVerifyWindowOnDesk(ctx context.Context, tconn *chrome.TestConn, ac *uiauto.Context) error {
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find all windows")
	}
	if len(ws) != 1 {
		return errors.Wrapf(err, "unexpected number of windows found; want: 1, got: %d", len(ws))
	}

	if err := clickToMoveWindow(ctx, tconn, ac, ws[0]); err != nil {
		return errors.Wrap(err, "failed to click and move window")
	}

	if err := verifyWindowOnAssignedDesk(ctx, tconn, ac, ws[0]); err != nil {
		return errors.Wrap(err, "failed to verify window on the assigned desk")
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
	if err := ac.LeftClick(moveToDesk2)(ctx); err != nil {
		return errors.Wrap(err, "failed to move the window to Desk 2")
	}

	if err := ash.WaitWindowFinishAnimating(ctx, tconn, w.ID); err != nil {
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

	return nil
}
