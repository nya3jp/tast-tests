// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package assistant

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/assistant"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OpenSettings,
		Desc:         "Tests opening the Settings app using an Assistant query",
		Contacts:     []string{"kyleshima@chromium.org", "bhansknecht@chromium.org", "meilinw@chromium.org"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          assistant.VerboseLoggingEnabled(),
	})
}

// OpenSettings tests that the Settings app can be opened by the Assistant
func OpenSettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	// Enable the Assistant and wait for the ready signal.
	if err := assistant.EnableAndWaitForReady(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Assistant: ", err)
	}
	defer assistant.Cleanup(ctx, s, cr, tconn)

	// Run query to open the Settings window.
	// assistant.SendTextQuery returns an error even when Settings launches successfully,
	// so check for that here instead of processing the returned error.
	// todo (crbug/1080366): process the error from assistantSendTextQuery.
	s.Log("Launching Settings app with Assistant query and waiting for it to open")
	_, assistErr := assistant.SendTextQuery(ctx, tconn, "open settings")
	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatalf("Settings app did not appear in the shelf: %v. Last assistant.SendTextQuery error: %v", err, assistErr)
	}

	// Confirm that the Settings app is open by checking for the "Settings" heading.
	params := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Settings",
	}
	if err := ui.WaitUntilExists(ctx, tconn, params, 10*time.Second); err != nil {
		s.Fatal("Waiting for Settings app heading failed: ", err)
	}
}
