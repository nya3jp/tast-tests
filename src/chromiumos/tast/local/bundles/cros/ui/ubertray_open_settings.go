// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UbertrayOpenSettings,
		Desc: "Checks that settings can be opened from the Ubertray",
		Contacts: []string{
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// UbertrayOpenSettings tests that we can open the settings menu from the Ubertray.
func UbertrayOpenSettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Find and click the Ubertray via UI.
	root, err := ui.Root(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get UI automation root: ", err)
	}
	defer root.Release(ctx)

	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	tray, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the status area (time, battery, etc.): ", err)
	}
	defer tray.Release(ctx)

	if err := tray.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click status area and open Ubertray: ", err)
	}

	// Find and click the Settings button in the Ubertray via UI.
	params = ui.FindParams{
		Name:      "Settings",
		ClassName: "TopShortcutButton",
	}
	settings, err := root.DescendantWithTimeout(ctx, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Ubertray Settings button: ", err)
	}
	defer settings.Release(ctx)

	if err := settings.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click Ubertray Settings button: ", err)
	}

	// Check that the Settings menu opened by looking for the menu and the shelf button.
	params = ui.FindParams{
		Name:      "Settings",
		ClassName: "BrowserFrame",
		Role:      ui.RoleTypeWindow,
	}
	if err := root.WaitForDescendant(ctx, params, true, 10*time.Second); err != nil {
		s.Fatal("Waiting for Settings window to open failed: ", err)
	}

	params = ui.FindParams{
		Name:      "Settings",
		ClassName: "ash/ShelfAppButton",
		Role:      ui.RoleTypeButton,
	}
	if err := root.WaitForDescendant(ctx, params, true, 10*time.Second); err != nil {
		s.Fatal("Waiting for Settings app button to appear failed: ", err)
	}
}
