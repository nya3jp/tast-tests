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

	if err := ash.ShowUbertrayWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to open the ubertray: ", err)
	}
	s.Log(ui.RootDebugInfo(ctx, tconn))

	if err := ash.OpenSettingsFromUbertray(ctx, tconn); err != nil {
		s.Fatal("Failed clicking Ubertray settings button: ", err)
	}

	// Wait for Settings app to open by checking if it's in the shelf.
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Settings app did not appear in the shelf: ", err)
	}

	// Confirm that the Settings app is open by checking for the "Settings" heading.
	params := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Settings",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 30*time.Second); err != nil {
		s.Fatal("Waiting for Settings app heading failed: ", err)
	}
}
