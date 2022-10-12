// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TabletModeBasicWindowControl,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tablet basics: Scroll, window controls",
		Contacts: []string{
			"shidi@chromium.org",
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
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func TabletModeBasicWindowControl(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

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

	// Open a new tab on current browser.
	connBlank, err := cr.NewConn(ctx, "http://youtube.com")
	if err != nil {
		s.Fatal("Failed to open new tab")
	}
	defer connBlank.Close()
	defer connBlank.CloseTarget(ctx)

	// Enter tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}

	defer cleanup(cleanupCtx)

	// Ensure that there is only one open window that is the primary browser.
	// Wait for the browser to be visible to avoid a race that may cause test flakiness.
	bt := s.Param().(browser.Type)
	bw, err := wmputils.EnsureOnlyBrowserWindowOpen(ctx, tconn, bt)
	if err != nil {
		s.Fatal("Failed to ensure only browser window open: ", err)
	}

	defer bw.CloseWindow(cleanupCtx, tconn)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	const uiTimeout = 5 * time.Second
	tc, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}

	// Scrolling with speed up fling.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	height := info.WorkArea.Height
	bigSwipeOffset := coords.NewPoint(0, height*9/10)
	// Sets swipe points and speed.
	BottomCenterPt := info.WorkArea.BottomCenter().Add(coords.NewPoint(1, 0))
	swipeStartPt := BottomCenterPt
	swipeEndPt := BottomCenterPt.Sub(bigSwipeOffset)
	const swipeSpeed = 10 * time.Millisecond
	const delay = 2 * time.Second

	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")

	// Wait for the window to finish animating before activating.
	if err := ash.WaitWindowFinishAnimating(ctx, tconn, bw.ID); err != nil {
		s.Fatal("Failed to wait for the window animation: ", err)
	}

	// Activate chrome window.
	if err := bw.ActivateWindow(ctx, tconn); err != nil {
		s.Fatal("Failed to activate chrome window: ", err)
	}

	ui := uiauto.New(tconn)

	// Make sure the address bar exist before scrolling.
	if err := ui.WithTimeout(uiTimeout).WaitUntilExists(addressBarNode.Onscreen())(ctx); err != nil {
		s.Fatal("Failed to find address bar: ", err)
	}

	// Scrolling up on chrome tab.
	if err := uiauto.Combine(
		"scrolling up on chrome tab", tc.Swipe(swipeStartPt, tc.SwipeTo(swipeEndPt, swipeSpeed), tc.Hold(time.Second)),
		// Address bar should should disappear during scrolling.
		ui.WithTimeout(uiTimeout).WaitUntilGone(addressBarNode.Onscreen()))(ctx); err != nil {
		s.Fatal("Failed to scroll up on browser window: ", err)
	}
	// Scrolling down on chrome tab.
	if err := uiauto.Combine(
		"scrolling down on chrome tab", tc.Swipe(swipeEndPt, tc.SwipeTo(swipeStartPt, swipeSpeed), tc.Hold(time.Second)),
		// Address bar should should reappear during scrolling.
		ui.WithTimeout(uiTimeout).WaitUntilExists(addressBarNode.Onscreen()))(ctx); err != nil {
		s.Fatal("Failed to scroll down on browser window: ", err)
	}
}
