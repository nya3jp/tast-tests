// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LauncherAndroidApps,
		Desc: "Launches an android app through the launcher",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"dhaddock@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
	})
}

func LauncherAndroidApps(ctx context.Context, s *testing.State) {
	const defaultTimeout = 15 * time.Second
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome
	if err := a.WaitIntentHelper(ctx); err != nil {
		s.Fatal("Failed to wait for ARC Intent Helper: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Get UI root.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to UI root: ", err)
	}
	defer root.Release(ctx)

	// Open Launcher.
	params := ui.FindParams{
		Name: "Launcher",
		Role: ui.RoleTypeButton,
	}
	launcherButton, err := root.DescendantWithTimeout(ctx, params, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to find launcher button: ", err)
	}
	defer launcherButton.Release(ctx)
	if err := launcherButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click launcher button: ", err)
	}

	// Click Search Box
	searchBox, err := root.DescendantWithTimeout(ctx, ui.FindParams{ClassName: "SearchBoxView"}, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to wait for launcher searchbox: ", err)
	}
	defer searchBox.Release(ctx)
	if err := searchBox.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click launcher searchbox: ", err)
	}

	// Setup keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Search for "Play Store"
	if err := kb.Type(ctx, "Play Store"); err != nil {
		s.Fatal("Failed to type 'Play Store': ", err)
	}

	// Wait for app icon and click it.
	params = ui.FindParams{
		Name:      "Play Store, Installed App",
		ClassName: "SearchResultTileItemView",
	}
	tile, err := root.DescendantWithTimeout(ctx, params, defaultTimeout)
	if err != nil {
		s.Fatal("Failed to wait for Play Store search result: ", err)
	}
	defer tile.Release(ctx)
	if err := tile.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click Play Store search result: ", err)
	}

	// Look at Shelf to make sure Play Store launched
	params = ui.FindParams{
		Name:      "Play Store",
		ClassName: "ash/ShelfAppButton",
	}
	if err := root.WaitForDescendant(ctx, params, true, defaultTimeout); err != nil {
		s.Fatal("Failed to wait for Play Store in Shelf: ", err)
	}
}
