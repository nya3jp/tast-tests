// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"

	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppFromSettings,
		Desc: "Help app can be launched from Settings",
		Contacts: []string{
			"showoff-eng@google.com",
			"carpenterr@chromium.org", // original test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{
			{
				Name:              "stable",
				ExtraHardwareDeps: pre.AppsStableModels,
			}, {
				Name:              "unstable",
				ExtraHardwareDeps: pre.AppsUnstableModels,
			},
		},
	})
}

// LaunchHelpAppFromSettings verifies launching Help app from Chrome OS settings.
func LaunchHelpAppFromSettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	conn, err := cr.NewConn(ctx, chrome.BlankURL)
	if err != nil {
		s.Fatal("Failed to lunch a new browser: ", err)
	}
	defer conn.Close()

	if err := helpapp.LaunchSettings(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Help app from settings: ", err)
	}

	// Find and click About Chrome OS.
	if err := helpapp.ClickElement(ctx, tconn, ui.FindParams{
		Name: "About Chrome OS",
		Role: ui.RoleTypeLink,
	}); err != nil {
		s.Fatal("Failed to click About Chrome OS: ", err)
	}

	// Find and click Get help with Chrome OS.
	if err := helpapp.ClickElement(ctx, tconn, ui.FindParams{
		Name: "Get help with Chrome OS",
		Role: ui.RoleTypeLink,
	}); err != nil {
		s.Fatal("Failed to click Get help with Chrome OS: ", err)
	}

	// App should be launched at the overview page.
	if err := helpapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch help app: ", err)
	}
}
