// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
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
		SoftwareDeps: []string{"android_both", "chrome"},
	})
}

func LauncherAndroidApps(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC by user policy: ", err)
	}
	defer a.Close()
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
		Role: "button",
	}
	launcherButton, err := root.GetDescendant(ctx, params)
	if err != nil {
		s.Fatal("Failed to find launcher button: ", err)
	}
	defer launcherButton.Release(ctx)
	if err := launcherButton.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click launcher button: ", err)
	}

	// Click Search Box
	params = ui.FindParams{
		Name: "Search your device, apps, and web. Use the arrow keys to navigate your apps.",
		Role: "textField",
	}
	searchBox, err := root.GetDescendantWithTimeout(ctx, params, 1*time.Second)
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
		Name:      "Play Store",
		ClassName: "SearchResultTileItemView",
	}
	icon, err := root.GetDescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to wait for Play Store Icon: ", err)
	}
	defer icon.Release(ctx)
	if err := icon.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click Play Store Icon: ", err)
	}

	// Look at Shelf to make sure Play Store launched
	params = ui.FindParams{
		Name:      "Play Store",
		ClassName: "ash/ShelfAppButton",
	}
	if err := root.WaitForDescendantAdded(ctx, params, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for Play Store in Shelf: ", err)
	}
}
