// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

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
		Desc:         "Checks that drag to split Chrome tabs works as intended",
		Contacts: []string{
			"sophiewen@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      2 * time.Minute,
		VarDeps:      []string{"ui.gaiaPoolDefault"},
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

	// Open 3 new Chrome tabs.
	conn1, err := cr.NewConn(ctx, "http://google.com")
	if err != nil {
		s.Fatal("Creating tab failed: ", err)
	}
	defer conn1.Close()
	conn2, err := cr.NewConn(ctx, "http://google.com")
	if err != nil {
		s.Fatal("Creating tab failed: ", err)
	}
	defer conn2.Close()
	conn3, err := cr.NewConn(ctx, "http://google.com")
	if err != nil {
		s.Fatal("Creating tab failed: ", err)
	}
	defer conn3.Close()

	// Gets primary display info and interesting drag points.
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
		s.Fatal("Failed to get the first tab: ", err)
	}

	snapRightPoint := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	// Drag the first tab in the tab strip and snap it to the right.
	if err := pc.Drag(firstTabRect.CenterPoint(),
		uiauto.Sleep(time.Second),
		pc.DragTo(snapRightPoint, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal(err, "failed to drag a tab to snap to the right")
	}

	// Drag the first tab from the left window and snap it to the left.
	secondTab := nodewith.Role(role.Tab).First()
	secondTabRect, err := ui.WithInterval(2*time.Second).Location(ctx, secondTab)
	if err != nil {
		s.Fatal("Failed to get the second tab: ", err)
	}
	snapLeftPoint := coords.NewPoint(info.WorkArea.Left, info.WorkArea.CenterY())
	if err := pc.Drag(secondTabRect.CenterPoint(),
		uiauto.Sleep(time.Second),
		pc.DragTo(snapLeftPoint, 3*time.Second),
	)(ctx); err != nil {
		s.Fatal("Failed to drag a tab to snap to the left: ", err)
	}
}
