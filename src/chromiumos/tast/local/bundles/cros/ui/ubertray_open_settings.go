// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: UbertrayOpenSettings,
		Desc: "Checks that settings can be opened from Quick Settings",
		Contacts: []string{
			"bhansknecht@chromium.org",
			"kyleshima@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// UbertrayOpenSettings tests that we can open the settings app from Quick Settings.
func UbertrayOpenSettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// TODO(crbug/1099502): replace this with quicksettings.Show when retry is no longer needed.
	if err := quicksettings.ShowWithRetry(ctx, tconn, 10*time.Second); err != nil {
		s.Fatal("Failed to open Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := quicksettings.OpenSettingsApp(ctx, tconn); err != nil {
		s.Fatal("Failed to open the Settings App from Quick Settings: ", err)
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
