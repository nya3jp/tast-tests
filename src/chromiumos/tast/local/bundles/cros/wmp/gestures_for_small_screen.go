// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/wmp/wmputils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GesturesForSmallScreen,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that gestures for hotseat, home, back and overview works correctly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func GesturesForSmallScreen(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Ensure there is no window open before test starts.
	if err := ash.CloseAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to ensure no window is open before test starts: ", err)
	}

	// Open a browser window either ash-chrome or lacros-chrome.
	browserApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find browser app info: ", err)
	}
	if err := apps.Launch(ctx, tconn, browserApp.ID); err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}

	// Ensure that there is only one open window that is the primary browser. Wait for the browser to be visible to avoid a race that may cause test flakiness.
	bt := s.Param().(browser.Type)
	bw, err := wmputils.EnsureOnlyBrowserWindowOpen(ctx, tconn, bt)
	if err != nil {
		s.Fatal("Expected the window to be fullscreen but got: ", err)
	}
	defer bw.CloseWindow(cleanupCtx, tconn)

	const uiTimeout = 5 * time.Second
	ac := uiauto.New(tconn)
	tc, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}

	shelf := nodewith.ClassName("ShelfWidget")
	shelfLoc, err := ac.Location(ctx, shelf)
	if err != nil {
		s.Fatal("Failed to get location of the shelf widget: ", err)
	}
	shelfCenterPt := shelfLoc.CenterPoint()

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	height, width := info.WorkArea.Height, info.WorkArea.Width
	// Offsets for swipe-ups.
	bigSwipeOffset := coords.NewPoint(0, height*9/10)
	smallSwipeOffset := coords.NewPoint(0, height*1/10)

	hotseat := nodewith.ClassName("HotseatWidget")
	appList := nodewith.ClassName("AppList")
	dropTargetView := nodewith.ClassName("DropTargetView")

	// Swipe up from shelf to open overview.
	if err := uiauto.Combine(
		"open overview",
		// Big swipe up and hold from shelf area opens overview screen.
		tc.Swipe(shelfCenterPt,
			tc.SwipeTo(shelfCenterPt.Sub(bigSwipeOffset),
				time.Second),
			tc.Hold(time.Second)),
		ac.WithTimeout(uiTimeout).WaitUntilExists(dropTargetView),
	)(ctx); err != nil {
		s.Fatal("Failed to open overview: ", err)
	}

	// Wait for the window to finish animating before activating.
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, bw.ID); err != nil {
		s.Fatal("Failed to wait for the window animation: ", err)
	}

	// Activate chrome window and exit from overview.
	if err := bw.ActivateWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to activate chrome window: ", err)
	}

	// Swipe up from shelf to open hotseat and home screen.
	if err := uiauto.Combine(
		"open hotseat and home screen",
		// Small swipe up from shelf area opens hotseat.
		tc.Swipe(shelfCenterPt,
			tc.SwipeTo(shelfCenterPt.Sub(smallSwipeOffset),
				200*time.Millisecond)),
		// Hotseat should show up.
		ac.WaitUntilExists(hotseat.Onscreen()),
		// Big swipe up from shelf area opens home screen.
		tc.Swipe(shelfCenterPt,
			tc.SwipeTo(shelfCenterPt.Sub(bigSwipeOffset),
				200*time.Millisecond)),
		// Home screen and app list should show up.
		ac.WithTimeout(uiTimeout).WaitUntilExists(appList.Visible()),
	)(ctx); err != nil {
		s.Fatal("Failed to open hotseat and home screen: ", err)
	}

	leftCenterPt := info.WorkArea.LeftCenter().Add(coords.NewPoint(1, 0))
	rightSwipeOffset := coords.NewPoint(width/4, 0)

	// Wait for the window to finish animating before activating.
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, bw.ID); err != nil {
		s.Fatal("Failed to wait for the window animation: ", err)
	}

	// Activate chrome window.
	if err := bw.ActivateWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to activate chrome window: ", err)
	}

	// Swipe from left edge to the right, to trigger the back gesture.
	if err := uiauto.Combine(
		"go back to the home screen",
		// Swipe from left side of the screen to the right.
		tc.Swipe(leftCenterPt,
			tc.SwipeTo(leftCenterPt.Add(rightSwipeOffset),
				time.Second),
			tc.Hold(time.Second)),
		// Should go back to the home screen.
		ac.WithTimeout(uiTimeout).WaitUntilExists(appList.Visible()),
	)(ctx); err != nil {
		s.Fatal("Failed to go back to the home screen: ", err)
	}
}
