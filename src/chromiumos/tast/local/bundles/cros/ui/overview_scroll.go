// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OverviewScroll,
		Desc:         "Checks that scrolling in tablet mode overview works properly",
		Contacts:     []string{"sammiequon@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "LoggedInDisableSync",
	})
}

func OverviewScroll(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the display rotation: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Prepare the touch screen as this test requires touch scroll events.
	tsew, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to create touch screen event writer: ", err)
	}
	defer tsew.Close()
	if err = tsew.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}

	stw, err := tsew.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	ac := uiauto.New(tconn)

	// Use a total of 16 windows for this test, so that scrolling can happen.
	const numWindows = 5
	// if err := ash.CreateWindows(ctx, tconn, cr, ui.PerftestURL, numWindows); err != nil {
	// 	s.Fatal("Failed to open browser windows: ", err)
	// }

	// Get the expected browser.
	chromeApp, err := apps.ChromeOrChromium(ctx, tconn)
	if err != nil {
		s.Fatal("Could not find the Chrome app: ", err)
	}

	appsList := []apps.App{chromeApp, apps.Files, apps.Gmail, apps.Docs, apps.Youtube}

	for _, app := range appsList {
		if err := apps.Launch(ctx, tconn, app.ID); err != nil {
			s.Fatalf("Failed to launch %s: %s", app.Name, err)
		}
		if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
			s.Fatalf("%s did not appear in shelf after launch: %s", app.Name, err)
		}
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}

	overviewItems := nodewith.ClassName("OverviewItemView")
	overviewItemsInfo, err := ac.NodesInfo(ctx, overviewItems)
	if err != nil {
		s.Fatal("Could not retrieve overview items: ", err)
	}

	if len(overviewItemsInfo) != numWindows {
		s.Fatalf("Unexpected number of overview items found; wanted %v, got %v", numWindows, len(overviewItemsInfo))
	}

	const numWindowsOffscreen = 0
	for _, overviewItem := range overviewItemsInfo {
		s.Log(overviewItem)
		// s.Log(overviewItem.State[chromeui.StateTypeFocused])
		// s.Log(ui.StateTypeFocused)
		// s.Log(overviewItem.State[ui.StateTypeFocused])
		// element is the element from someSlice for where we are
	}

	// Scroll from the top right of the screen to the top middle (1/4 of the
	// screen width). The destination position should match with the next swipe
	// to make the same amount of scrolling.
	if err := stw.Swipe(ctx, tsew.Width()-10, 10, tsew.Width()/4, 10, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to execute a swipe gesture: ", err)
	}

	if err := stw.End(); err != nil {
		s.Fatal("Failed to finish the swipe gesture: ", err)
	}

	// Scroll back from the top middle to the top right so that the test returns
	// back to the original status. Note that this can't be starting from the
	// top left, since it can be recognized as another gesture (back gesture).
	if err := stw.Swipe(ctx, tsew.Width()/4, 10, tsew.Width()-10, 10, 500*time.Millisecond); err != nil {
		s.Fatal("Failed to execute a swipe gesture: ", err)
	}
	if err := stw.End(); err != nil {
		s.Fatal("Failed to finish the swipe gesture: ", err)
	}

	if err = ash.SetOverviewModeAndWait(ctx, tconn, false); err != nil {
		s.Fatal("It does not appear to be in the overview mode: ", err)
	}
}
