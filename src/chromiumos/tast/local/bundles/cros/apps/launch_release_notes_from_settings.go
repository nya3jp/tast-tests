// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/apps/helpapp"
	"chromiumos/tast/local/bundles/cros/apps/pre"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchReleaseNotesFromSettings,
		Desc: "Help app release notes can be launched from Settings",
		Contacts: []string{
			"carpenterr@chromium.org", // test author.
			"showoff-eng@google.com",
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

// LaunchReleaseNotesFromSettings verifies launching Help app at the release notes page from Chrome OS settings.
func LaunchReleaseNotesFromSettings(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS); err != nil {
		s.Fatal("Failed to launch Settings: ", err)
	}

	// Find and click See what's new.
	if err := ui.FindAndClick(ctx, tconn, ui.FindParams{
		Name: "See what's new",
		Role: ui.RoleTypeLink,
	}, 10*time.Second); err != nil {
		s.Fatal("Failed to click See whats new: ", err)
	}

	// Wait for the Help app to load.
	helpRootNode, err := helpapp.HelpRootNode(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to find help app: ", err)
	}
	// Wait for the heading of What's new to be rendered to verify tab.
	if _, err := helpRootNode.DescendantWithTimeout(ctx, ui.FindParams{
		Name: "What’s new with your Chromebook?",
		Role: ui.RoleTypeHeading,
	}, 20*time.Second); err != nil {
		s.Fatal("Failed to find updates page heading: ", err)
	}
}
