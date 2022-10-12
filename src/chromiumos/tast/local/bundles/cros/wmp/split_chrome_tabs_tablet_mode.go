// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SplitChromeTabsTabletMode,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Chrome tabs are draggable to split screen",
		Contacts: []string{
			"sophiewen@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func SplitChromeTabsTabletMode(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure tablet mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Open a new Chrome window with three tabs.
	url := "http://google.com"
	for i := 0; i < 3; i++ {
		conn, err := cr.NewConn(ctx, url)
		if err != nil {
			s.Fatalf("Failed to open new tab with url: %s, %v", url, err)
		}
		defer conn.Close()
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	pc, err := pointer.NewTouch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create a touch controller: ", err)
	}

	// Open the Chrome browser tab strip.
	tabStripButton := nodewith.Role(role.Button).ClassName("WebUITabCounterButton").First()
	if err := pc.Click(tabStripButton)(ctx); err != nil {
		s.Fatal("Failed to click the tab strip button: ", err)
	}

	ui := uiauto.New(tconn)

	// Get the first tab location with a polling interval of 2 seconds (meaning
	// wait until the location is stable for 2 seconds) to work around a
	// glitchy animation that sometimes happens when bringing up the tab strip.
	firstTab := nodewith.Role(role.Tab).First()
	firstTabRect, err := ui.WithInterval(2*time.Second).Location(ctx, firstTab)
	if err != nil {
		s.Fatal("Failed to get the first tab strip thumbnail: ", err)
	}

	// Drag the first tab in the tab strip, sleep to long press the tab, and snap it to the right.
	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())
	if err := pc.Drag(firstTabRect.CenterPoint(),
		action.Sleep(time.Second),
		pc.DragTo(snapRightPoint, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag the first tab to the right: ", err)
	}

	// Ensure the tab has been dragged to a new window and snapped right.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	if len(ws) != 2 || ws[0].State != ash.WindowStateRightSnapped {
		s.Fatal("Failed to snap the first tab to the right")
	}

	// The first tab is now in the left window.
	leftTabRect, err := ui.WithInterval(2*time.Second).Location(ctx, firstTab)
	if err != nil {
		s.Fatal("Failed to get the second tab strip thumbnail: ", err)
	}

	// Drag the first tab in the left window, sleep to long press the tab, and snap it to the left.
	snapLeftPoint := coords.NewPoint(info.WorkArea.Left, info.WorkArea.CenterY())
	if err := pc.Drag(leftTabRect.CenterPoint(),
		action.Sleep(time.Second),
		pc.DragTo(snapLeftPoint, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag the second tab to the left: ", err)
	}

	// Ensure the tab has been dragged to a new window and is snapped left.
	ws, err = ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the window list: ", err)
	}
	if len(ws) != 3 || ws[0].State != ash.WindowStateLeftSnapped {
		s.Fatal("Failed to snap the window to the left")
	}
}
