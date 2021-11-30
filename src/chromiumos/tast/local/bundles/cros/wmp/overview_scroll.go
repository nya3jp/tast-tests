// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func:         OverviewScroll,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that scrolling in tablet mode overview works properly",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Params: []testing.Param{{
			Name: "portrait",
			Val:  true,
		}, {
			Name: "landscape",
			Val:  false,
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 120*time.Second,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

func OverviewScroll(ctx context.Context, s *testing.State) {
	// Reserve five seconds for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(cleanupCtx)

	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ac := uiauto.New(tconn)

	// Rotate the screen if it is a portrait test.
	portrait := s.Param().(bool)
	if portrait {
		info, err := display.GetInternalInfo(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to obtain internal display info: ", err)
		}
		if err = display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, info.ID, display.Rotate0)
	}

	// Use a total of 16 windows for this test, so that scrolling can happen.
	const numWindows = 16

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

	// Create some chrome apps that are already installed.
	appsList := []apps.App{apps.Camera, apps.Files, apps.Help, apps.PlayStore}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	// Create enough chrome windows so that we have 16 total windows.
	numChromeWindows := numWindows - len(appsList)
	if err := ash.CreateWindows(ctx, tconn, cr, "", numChromeWindows); err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}

	// There should be max 8 onscreen windows at any time; the rest should be offscreen.
	const maxNumOnscreenWindows = 8

	// Verify the windows; the first two windows, which are the first two launched windows should be offscreen.
	if err := verifyOverviewItemsInfo(ctx, ac, numWindows, maxNumOnscreenWindows, true); err != nil {
		s.Fatal("Failed to verify overview items info: ", err)
	}

	tc, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	// Swipe from right edge to the left a couple times to scroll the overview grid.
	swipeStartPt := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
	swipeEndPt := coords.NewPoint(0, info.WorkArea.CenterY())
	const delay = 500 * time.Millisecond
	if err := uiauto.Combine(
		"scroll overview",
		tc.Swipe(swipeStartPt,
			tc.SwipeTo(swipeEndPt,
				delay),
			tc.Hold(delay)),
		tc.Swipe(swipeStartPt,
			tc.SwipeTo(swipeEndPt,
				delay),
			tc.Hold(delay)),
		tc.Swipe(swipeStartPt,
			tc.SwipeTo(swipeEndPt,
				delay),
			tc.Hold(delay)),
	)(ctx); err != nil {
		s.Fatal("Failed to go scroll through overview items: ", err)
	}

	// Verify the windows again; the first two windows should now be onscreen.
	if err := verifyOverviewItemsInfo(ctx, ac, numWindows, maxNumOnscreenWindows, false); err != nil {
		s.Fatal("Failed to verify overview items info: ", err)
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}
}

// verifyOverviewItemsInfo checks various expected behaviors involving overview
// items while scrolling. expectedItems is the amount of items expected; it
// should match the amount of app windows created. maxNumOnscreen is the max
// number of windows that are visible to the user. It should be six usually,
// though depending on the scroll phase, it can be up to eight. We also do a
// check on the offscreen value of the first two windows; they should match
// expectedFirstTwoItemsOffscreen.
func verifyOverviewItemsInfo(ctx context.Context, ac *uiauto.Context, expectedItems, maxNumOnscreen int, expectedFirstTwoItemsOffscreen bool) error {
	overviewItems := nodewith.HasClass("OverviewItemView")
	overviewItemsInfo, err := ac.NodesInfo(ctx, overviewItems)
	if err != nil {
		return errors.Wrap(err, "could not retrieve overview items")
	}

	if len(overviewItemsInfo) != expectedItems {
		return errors.Errorf("unexpected number of overview items found; wanted %v, got %v", expectedItems, len(overviewItemsInfo))
	}

	// Count the number of onscreen windows.
	numWindowsOnscreen := 0
	for _, overviewItem := range overviewItemsInfo {
		if !overviewItem.State["offscreen"] {
			numWindowsOnscreen++
		}
	}

	if numWindowsOnscreen > maxNumOnscreen {
		return errors.Errorf("unexpected number of overview items onscreen; wanted at most %v, got %v", maxNumOnscreen, numWindowsOnscreen)
	}

	// Verify whether the first two windows, which are the LRU windows matches the expected offscreen value.
	if overviewItemsInfo[0].State["offscreen"] != expectedFirstTwoItemsOffscreen {
		return errors.Errorf("window 1 does not match the expected offscreen value, which is %v", expectedFirstTwoItemsOffscreen)
	}

	if overviewItemsInfo[1].State["offscreen"] != expectedFirstTwoItemsOffscreen {
		return errors.Errorf("window 2 does not match the expected offscreen value, which is %v", expectedFirstTwoItemsOffscreen)
	}

	return nil
}
