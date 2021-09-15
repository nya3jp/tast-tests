// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/errors"
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
		Func:         OverviewScroll,
		Desc:         "Checks that scrolling in tablet mode overview works properly",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "portrait",
			Val:  true,
		}, {
			Name: "landscape",
			Val:  false,
		}},
	})
}

// getOverviewItemsInfo checks that the expected number of overview items are created, and that the expected number is onscreen.
// Returns the info of the overview items and an error message if any.
func getOverviewItemsInfo(ctx context.Context, ac *uiauto.Context, expectedItems, maxOnscreenCount int) ([]uiauto.NodeInfo, error) {
	overviewItems := nodewith.ClassName("OverviewItemView")
	overviewItemsInfo, err := ac.NodesInfo(ctx, overviewItems)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve overview items")
	}

	if len(overviewItemsInfo) != expectedItems {
		return nil, errors.Errorf("unexpected number of overview items found; wanted %v, got %v", expectedItems, len(overviewItemsInfo))
	}

	// Count the number of onscreen windows.
	numWindowsOnscreen := 0
	for _, overviewItem := range overviewItemsInfo {
		if !overviewItem.State["offscreen"] {
			numWindowsOnscreen++
		}
	}

	if numWindowsOnscreen > maxOnscreenCount {
		return nil, errors.Errorf("unexpected number of overview items onscreen; wanted at most %v, got %v", maxOnscreenCount, numWindowsOnscreen)
	}

	return overviewItemsInfo, nil
}

func OverviewScroll(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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
		defer display.SetDisplayRotationSync(ctx, tconn, info.ID, display.Rotate0)
	}

	// Use a total of 16 windows for this test, so that scrolling can happen.
	const numWindows = 16

	// Create some chrome apps that are already installed.
	appsList := []apps.App{apps.Camera, apps.Files, apps.Help}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	// TODO(sammiequon): Add support for ARC apps.

	// Create enough chrome windows so that we have 16 total windows.
	numChromeWindows := numWindows - len(appsList)
	if err := ash.CreateWindows(ctx, tconn, cr, "", numChromeWindows); err != nil {
		s.Fatal("Failed to create new windows: ", err)
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}

	// There should be max 8 onscreen windows at any time; the rest should be offscreen.
	const maxOnscreenWindows = 8

	overviewItemsInfo, err := getOverviewItemsInfo(ctx, ac, numWindows, maxOnscreenWindows)
	if err != nil {
		s.Fatal("Failed to get overview items info: ", err)
	}

	// The first two windows, which are the LRU windows should be offscreen.
	if !overviewItemsInfo[0].State["offscreen"] || !overviewItemsInfo[1].State["offscreen"] {
		s.Fatal("One of the first two windows is onscreen")
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

	// Get the new overview items info.
	overviewItemsInfo, err = getOverviewItemsInfo(ctx, ac, numWindows, maxOnscreenWindows)
	if err != nil {
		s.Fatal("Failed to get overview items info: ", err)
	}

	// The first two windows, which are the LRU windows should now be onscreen.
	if overviewItemsInfo[0].State["offscreen"] || overviewItemsInfo[1].State["offscreen"] {
		s.Fatal("One of the first two windows is offscreen")
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}
}
