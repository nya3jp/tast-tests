// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UbertrayOpenSettings,
		Desc: "Checks that settings can be opened from the Ubertray",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// UbertrayOpenSettings tests that we can open the settings app from the Ubertray.
func UbertrayOpenSettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the status area (time, battery, etc.): ", err)
	}
	defer statusArea.Release(ctx)

	// Find and click the Settings button in the Ubertray via UI.
	params = ui.FindParams{
		Name:      "Settings",
		ClassName: "TopShortcutButton",
	}

	// Sometimes the left-click to the status area can happen too quickly,
	// so the status area doesn't receive the click and the Ubertray doesn't open.
	// To prevent this, we can repeat the click until the Ubertray opens.
	// todo(crbug/1099502): determine when this is clickable, and just click it once.
	condition := func(ctx context.Context) (bool, error) {
		return ui.Exists(ctx, tconn, params)
	}
	opts := testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}
	if err := statusArea.LeftClickUntil(ctx, condition, &opts); err != nil {
		s.Fatal("Failed to click the status area and find the Ubertray Settings button: ", err)
	}

	settingsBtn, err := ui.FindWithTimeout(ctx, tconn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find Ubertray Settings button: ", err)
	}
	defer settingsBtn.Release(ctx)

	// Try clicking the Settings button until it goes away, indicating the click was received.
	// todo(crbug/1099502): determine when this is clickable, and just click it once.
	condition = func(ctx context.Context) (bool, error) {
		exists, err := ui.Exists(ctx, tconn, params)
		return !exists, err
	}
	if err := settingsBtn.LeftClickUntil(ctx, condition, &opts); err != nil {
		s.Fatal("Settings button still present after clicking it repeatedly: ", err)
	}

	// Wait for Settings app to open by checking if it's in the shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Settings app did not appear in the shelf: ", err)
	}

	// Confirm that the Settings app is open by checking for the "Settings" heading.
	params = ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Settings",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 30*time.Second); err != nil {
		s.Fatal("Waiting for Settings app heading failed: ", err)
	}
}
