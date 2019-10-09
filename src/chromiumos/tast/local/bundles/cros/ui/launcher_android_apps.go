// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
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
		SoftwareDeps: []string{"android", "chrome"},
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

	// Open Launcher.
	params := ui.FindParams{
		Attributes: map[string]interface{}{"name": "Launcher", "role": "button"},
	}
	if err := ui.LeftClick(ctx, tconn, params); err != nil {
		s.Fatal("Failed to click launcher button: ", err)
	}

	// Click Search Box
	params = ui.FindParams{
		Attributes: map[string]interface{}{"name": "Search your device, apps, and web. Use the arrow keys to navigate your apps.", "role": "textField"},
	}
	if err := ui.WaitForNodeToAppear(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for launcher searchbox: ", err)
	}
	if err := ui.LeftClick(ctx, tconn, params); err != nil {
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

	// Wait for app icon.
	params = ui.FindParams{
		Attributes: map[string]interface{}{"name": "Play Store", "className": "SearchResultTileItemView"},
	}
	if err := ui.WaitForNodeToAppear(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for Play Store icon: ", err)
	}

	// Launch Play Store.
	if err := ui.LeftClick(ctx, tconn, params); err != nil {
		s.Fatal("Failed to click Play Store Icon: ", err)
	}

	// Look at Shelf to make sure Play Store launched
	params = ui.FindParams{
		Attributes: map[string]interface{}{"name": "Play Store", "className": "ash/ShelfAppButton"},
	}
	if err := ui.WaitForNodeToAppear(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for Play Store in Shelf: ", err)
	}
}
