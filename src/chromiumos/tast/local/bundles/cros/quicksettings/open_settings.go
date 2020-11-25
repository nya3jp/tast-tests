// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quicksettings

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/local/chrome/ui/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenSettings,
		Desc: "Checks that settings can be opened from Quick Settings",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"amehfooz@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

// OpenSettings tests that we can open the settings app from Quick Settings.
func OpenSettings(ctx context.Context, s *testing.State) {
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

	// Confirm that the Settings app is open by checking for the search box.
	if err := ossettings.WaitForSearchBox(ctx, tconn, 30*time.Second); err != nil {
		s.Fatal("Waiting for Settings app search box failed: ", err)
	}
}
