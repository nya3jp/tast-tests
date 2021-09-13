// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutohideShelf,
		Desc:         "Verify the functionality of autohide shelf",
		Contacts:     []string{"cienet-development@googlegroups.com", "lance.wang@cienet.com"},
		Fixture:      "arcBootedWithPlayStore",
		SoftwareDeps: []string{"chrome", "arc"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      5 * time.Minute,
	})
}

type shelfMode int

const (
	autoHide shelfMode = iota
	alwaysShow
)

// AutohideShelf verifies the functionality of hiding shelf.
func AutohideShelf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer tconn.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	}(cleanupCtx)

	a := s.FixtValue().(*arc.PreData).ARC
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(cleanupCtx)

	testing.ContextLog(ctx, "Enable autohide feature")
	if err := setShelf(ctx, tconn, autoHide); err != nil {
		s.Fatal("Failed to enable autohide feature: ", err)
	}
	defer setShelf(cleanupCtx, tconn, alwaysShow)

	chromeBrowserApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the `Chrome` app: ", err)
	}

	testing.ContextLogf(ctx, "Launch %q", chromeBrowserApp.Name)
	if err := launchAppAndVerifyShelf(ctx, tconn, chromeBrowserApp, ash.ShelfShownClamShell); err != nil {
		s.Fatal("Failed to launch app `Chrome` and verify shelf status: ", err)
	}
	defer apps.Close(cleanupCtx, tconn, apps.Chrome.ID)

	testing.ContextLog(ctx, "Enable tablet mode")
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to activate tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	testing.ContextLog(ctx, "Make a small swipe up")
	if err := showHotseat(ctx, tconn); err != nil {
		s.Fatal("Failed to show hotseat: ", err)
	}

	testing.ContextLog(ctx, "Enter overview mode")
	if err := swipeUpAndLift(ctx, tconn); err != nil {
		s.Fatal("Failed to enter overview mode: ", err)
	}

	gamingApp := apps.App{
		ID:   "com.budgestudios.HelloKittyLunchBox",
		Name: "Lunchbox",
	}

	testing.ContextLogf(ctx, "Install gaming app %q", gamingApp.Name)
	if err := playstore.InstallApp(ctx, a, d, gamingApp.ID, -1); err != nil {
		s.Fatalf("Failed to install app %q: %v", gamingApp.Name, err)
	}
	defer apps.Close(cleanupCtx, tconn, apps.PlayStore.ID)

	testing.ContextLogf(ctx, "Launch gaming app %q", gamingApp.Name)
	if err := launchAppAndVerifyShelf(ctx, tconn, gamingApp, ash.ShelfHidden); err != nil {
		s.Fatalf("Failed to launch app %q and verify shelf status: %v", gamingApp.Name, err)
	}
	defer func(ctx context.Context) {
		w, err := ash.GetARCAppWindowInfo(ctx, tconn, gamingApp.ID)
		if err != nil {
			testing.ContextLogf(ctx, "Unable to locate the window with package ID %q: %v", gamingApp.ID, err)
		}
		if err := w.CloseWindow(ctx, tconn); err != nil {
			testing.ContextLogf(ctx, "Unable to clise the window with package ID %q: %v", gamingApp.ID, err)
		}
	}(cleanupCtx)
}

// setShelf turns on/off of shelf autohide functionality.
func setShelf(ctx context.Context, tconn *chrome.TestConn, mode shelfMode) error {
	var modes = map[shelfMode]string{
		autoHide:   "Autohide shelf",
		alwaysShow: "Always show shelf",
	}

	ui := uiauto.New(tconn)
	windowObject := nodewith.ClassName("WallpaperView")

	menuItem := nodewith.Role(role.MenuItem).First()

	if err := ui.WithTimeout(time.Minute).WithInterval(3*time.Second).RightClickUntil(windowObject, ui.WaitUntilExists(menuItem))(ctx); err != nil {
		return errors.Wrap(err, "failed to right-click and open menu")
	}

	if err := ui.LeftClick(nodewith.Name(modes[mode]).ClassName("MenuItemView"))(ctx); err != nil {
		testing.ContextLogf(ctx, "Cannot click %q, it might already be triggered", modes[mode])
	}

	shelfStatus, err := ash.FetchHotseatInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get shelf status")
	}

	return ash.WaitForHotseatToUpdateAutoHideState(ctx, tconn, shelfStatus.IsAutoHidden)
}

// verifyShelfStatus checks shelf status.
func verifyShelfStatus(ctx context.Context, tconn *chrome.TestConn, status ash.HotseatStateType) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		if err := ash.WaitForHotseatAnimationToFinish(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to wait for hoseat to finish")
		}

		info, err := ash.FetchHotseatInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to fetch hotseat information")
		}

		if info.HotseatState != status {
			return errors.Errorf("failed to verify shelf status: (want: %q; got %q)", status, info.HotseatState)
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second})
}

// launchAppAndVerifyShelf launches the given app and verifies shelf status.
func launchAppAndVerifyShelf(ctx context.Context, tconn *chrome.TestConn, app apps.App, status ash.HotseatStateType) error {
	if err := launcher.LaunchApp(tconn, app.Name)(ctx); err != nil {
		return errors.Wrapf(err, "failed to run application %q from application list view: ", app)
	}

	return verifyShelfStatus(ctx, tconn, status)
}

// showHotseat performs slight-swipeup action anv verifies shelf status under tablet mode.
func showHotseat(ctx context.Context, tconn *chrome.TestConn) error {
	if err := ash.ShowHotseat(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to swipe up the hotseat to show extended shelf")
	}
	return verifyShelfStatus(ctx, tconn, ash.ShelfExtended)
}

// swipeUpAndLift simulates swipe up and lift actions and verifies shelf status.
func swipeUpAndLift(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	tsew, _, err := touch.NewTouchscreenAndConverter(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to access the touchscreen")
	}
	defer tsew.Close()
	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		return errors.Wrap(err, "failed to create the single touch writer")
	}

	window, err := ui.Info(ctx, nodewith.ClassName("WallpaperView"))

	// x shoud be slightly right from the most-left point.
	x := input.TouchCoord(window.Location.Width / 5)
	// y shoud be at the point of 1/3 height of the screen to ensure shelf goes into overview mode.
	y := input.TouchCoord((window.Location.Top + window.Location.Height/3))

	if err := stw.Swipe(ctx, x, 0, x, y, time.Second); err != nil {
		return errors.Wrap(err, "failed to swipe")
	}

	if err := stw.End(); err != nil {
		return errors.Wrap(err, "failed to lift")
	}

	if err := ui.WaitUntilExists(nodewith.ClassName("OverviewItemView"))(ctx); err != nil {
		return errors.Wrap(err, "failed to wait object `OverviewItemView` exist")
	}

	return verifyShelfStatus(ctx, tconn, ash.ShelfExtended)
}
