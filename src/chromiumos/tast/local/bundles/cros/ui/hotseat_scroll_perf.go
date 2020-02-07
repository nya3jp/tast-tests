// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HotseatScrollPerf,
		Desc: "Records the animation smoothness for shelf scroll animation",
		Contacts: []string{
			"andrewxu@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          ash.LoggedInWith100DummyApps(),
		// Data:         []string{"16_gibbon.png", "44_gibbon.png", "64_gibbon.png", "128_gibbon.png"},
	})
}

// directionType specifies
type directionType int

const (
	left directionType = iota
	right
)

func scrollAlongOneDirectionUntilEnd(ctx context.Context, tconn *chrome.Conn, d directionType, root *(ui.Node)) error {
	arrowButtonClassName := "ScrollableShelfArrowView"
	icons, err := root.Descendants(ctx, ui.FindParams{ClassName: arrowButtonClassName})

	// Assumes that at the beginning there should be one and only one arrow button.
	if len(icons) != 1 || err != nil {
		return err
	}

	arrowButton := icons[0]

	for {
		if err := arrowButton.LeftClick(ctx); err != nil {
			return err
		}

		// Waits for UI refresh.
		if err := testing.Sleep(ctx, 1*time.Second); err != nil {
			return err
		}

		icons, err = root.Descendants(ctx, ui.FindParams{ClassName: arrowButtonClassName})

		// Keeps clicking when there are two arrow buttons showing.
		if len(icons) != 2 {
			break
		}

		// Chooses the suitable arrow button to click based on the scroll direction.
		arrowButton = icons[0]
		x0 := icons[0].Location.Left
		x1 := icons[1].Location.Left
		if (d == left && x0 > x1) || (d == right && x0 < x1) {
			arrowButton = icons[1]
		}
	}

	return nil
}

func runShelfScrollAnimation(ctx context.Context, tconn *chrome.Conn) error {
	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		return err
	}
	defer root.Release(ctx)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return err
	}
	if err := scrollAlongOneDirectionUntilEnd(ctx, tconn, right, root); err != nil {
		return err
	}
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return err
	}
	if err := scrollAlongOneDirectionUntilEnd(ctx, tconn, left, root); err != nil {
		return err
	}

	return nil
}

// HotseatScrollPerf ...???
func HotseatScrollPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	{
		// At login, we should have just Chrome in the Shelf.
		shelfItems, err := ash.ShelfItems(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get shelf items: ", err)
		}
		if len(shelfItems) != 1 {
			s.Fatal("Unexpected apps in the shelf. Expected only Chrome: ", shelfItems)
		}
	}

	// Pins additional 30 apps to Shelf.
	installedApps, err := ash.ChromeApps(ctx, tconn)
	for i := 0; i < 30; i++ {
		if err := ash.PinApp(ctx, tconn, installedApps[i].AppID); err != nil {
			s.Fatalf("Failed to launch %s: %s", installedApps[i].AppID, err)
		}
	}

	if err := runShelfScrollAnimation(ctx, tconn); err != nil {
		s.Fatalf("Fail to run scroll animation %s", err)
	}
}
