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
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchHelpAppWhatsNewTab,
		Desc: "Checks that Help app's What's New tab can be launched from the Settings app",
		Contacts: []string{
			"showoff-eng@google.com",
			"shengjun@chromium.org",
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
		}})
}

// LaunchHelpAppWhatsNewTab tests that we can open the What's New tab of the Help app from the Settings app entry point.
func LaunchHelpAppWhatsNewTab(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Wait for the Help App to be available.
	if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Help.ID, 10*time.Second); err != nil {
		s.Fatal("Failed waiting for Help app to be installed: ", err)
	}

	// Launch the Settings app and wait for it to open.
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Failed to launch the Settings app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		s.Fatal("Settings app did not appear in the shelf: ", err)
	}

	// Establish a Chrome connection to the Settings app and wait for it to finish loading.
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://os-settings/"))
	if err != nil {
		s.Fatal("Failed to get Chrome connection to Settings app: ", err)
	}
	defer settingsConn.Close()

	if err := settingsConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		s.Fatal("Failed waiting for Settings app document state to be ready: ", err)
	}

	// Wait for settings API to be available.
	if err := settingsConn.WaitForExpr(ctx, `settings !== undefined`); err != nil {
		s.Fatal("Failed waiting for settings API to load: ", err)
	}

	// Open Help app to the What's New tab using the Settings page JS functions. The same JS is tied to the UI link's on-click property.
	if err := settingsConn.Eval(ctx,
		"settings.AboutPageBrowserProxyImpl.getInstance().launchReleaseNotes()",
		nil); err != nil {
		s.Fatal("Failed to run Javascript to open Help app to What's New tab: ", err)
	}

	// Wait for the Help app to open.
	if err := helpapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed waiting for help app: ", err)
	}

	if err := helpapp.WaitWhatsNewTabRendered(ctx, tconn); err != nil {
		s.Error("Failed to verify what's new tab rendering: ", err)
	}
}
