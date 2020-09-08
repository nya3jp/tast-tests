// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppFromLauncher,
		Desc: "Help app can be found and launched from the launcher",
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

// LaunchHelpAppFromLauncher verifies launching Help app from the launcher.
func LaunchHelpAppFromLauncher(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := launcher.OpenLauncher(ctx, tconn); err != nil {
		s.Fatal("Failed to open launcher")
	}

	// Try searching for "Get help"
	if err := launcher.Search(ctx, tconn, "Get help"); err != nil {
		s.Fatal("Failed to search for get help")
	}

	// Help app should be one of the search results.
	appNode, err := launcher.WaitForAppResult(ctx, tconn, apps.Help.Name, 15*time.Second)
	if err != nil {
		s.Fatal("Help app does not exist in search result")
	}

	// Clicking that result should open the help app.
	if err := appNode.LeftClick(ctx); err != nil {
		s.Fatal("Failed to launch app from search result")
	}

	// App should be launched at the overview page.
	if err := helpapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch help app: ", err)
	}
}
