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
	"chromiumos/tast/local/chrome/uiauto/role"
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

	// Launch Settings.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to open Settings: ", err)
	}

	ac := uiauto.New(tconn)

	frameToolbarViewFinder := nodewith.HasClass("WebAppFrameToolbarView")
	searchBoxFinder := nodewith.Role(role.SearchBox)
	// Wait until the search box shows up.
	if err := ac.WaitForLocation(searchBoxFinder)(ctx); err != nil {
		s.Fatal("Failed to wait for the search box: ", err)
	}

	oldInfo, err := ac.Info(ctx, frameToolbarViewFinder)
	if err != nil {
		s.Fatal("Failed to find frame toolbar view: ", err)
	}
	oldBounds := oldInfo.Location

	start := oldBounds.CenterPoint()
	end := start.Add(coords.NewPoint(100, 100))
	if err := mouse.Drag(tconn, start, end, time.Second)(ctx); err != nil {
		s.Fatal("Failed to drag window: ", err)
	}

	newInfo, err := ac.Info(ctx, frameToolbarViewFinder)
	if err != nil {
		s.Fatal("Failed to find frame toolbar view: ", err)
	}
	newBounds := newInfo.Location
	// Frame toolbar view bounds as well as the window bounds should change after the drap and drop.
	if oldBounds == newBounds {
		s.Fatal("Drag failed: window bounds didn't change")
	}
}
