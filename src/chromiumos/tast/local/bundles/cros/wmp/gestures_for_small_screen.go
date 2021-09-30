// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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
		Func: GesturesForSmallScreen,
		Desc: "Checks that gestures for hotseat, home, back and overview works correctly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func GesturesForSmallScreen(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
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

	// Open a chrome window.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch chrome: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the window list: ", err)
	}
	if len(ws) != 1 {
		s.Fatal("Expected 1 window, got ", len(ws))
	}

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
	overviewModeLabel := nodewith.ClassName("OverviewModeLabel")

	// Swipe up from shelf to open overview.
	if err := uiauto.Combine(
		"open overview",
		// Big swipe up and hold from shelf area opens overview screen.
		tc.Swipe(shelfCenterPt,
			tc.SwipeTo(shelfCenterPt.Sub(bigSwipeOffset),
				time.Second),
			tc.Hold(time.Second)),
		ac.WithTimeout(uiTimeout).WaitUntilExists(overviewModeLabel),
	)(ctx); err != nil {
		s.Fatal("Failed to open overview: ", err)
	}

	// Activate chrome window and exit from overview.
	if err := ws[0].ActivateWindow(ctx, tconn); err != nil {
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
	// Activate chrome window.
	if err := ws[0].ActivateWindow(ctx, tconn); err != nil {
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
