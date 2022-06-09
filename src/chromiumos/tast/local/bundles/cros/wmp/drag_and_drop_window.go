// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wmp

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DragAndDropWindow,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that window drag and drop works correctly and smoothly",
		Contacts: []string{
			"yichenz@chromium.org",
			"chromeos-wmp@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func DragAndDropWindow(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Launch Chrome window.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to open Chrome: ", err)
	}

	windows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	if len(windows) != 1 {
		s.Fatalf("Expected 1 window; got %d", len(windows))
	}
	if err := ash.SetWindowStateAndWait(ctx, tconn, windows[0].ID, ash.WindowStateNormal); err != nil {
		s.Fatal("Failed to set chrome window state to normal: ", err)
	}

	ac := uiauto.New(tconn)

	oldInfo, err := ac.Info(ctx, nodewith.HasClass("TabStripRegionView"))
	if err != nil {
		s.Fatal("Failed to find TabStripRegionView: ", err)
	}
	oldBounds := oldInfo.Location

	start := oldBounds.CenterPoint()
	end := start.Add(coords.NewPoint(100, 100))
	if err := mouse.Drag(tconn, start, end, time.Second)(ctx); err != nil {
		s.Fatal("Failed to drag window: ", err)
	}

	newInfo, err := ac.Info(ctx, nodewith.HasClass("TabStripRegionView"))
	if err != nil {
		s.Fatal("Failed to find TabStripRegionView: ", err)
	}
	newBounds := newInfo.Location
	// TabStripRegionView bounds as well as window bounds should change after the drap and drop.
	if oldBounds == newBounds {
		s.Fatal("Drag failed: window bounds didn't change")
	}
}
