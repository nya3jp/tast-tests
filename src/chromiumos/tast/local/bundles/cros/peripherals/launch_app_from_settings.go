// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/bundles/cros/peripherals/peripheraltypes"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/ossettings"
	"chromiumos/tast/testing"
)

// settingsTestParams contains all the data needed to run a single test iteration.
type settingsTestParams struct {
	appName      string
	featureFlag  string
	waitForApp   peripheraltypes.WaitForAppFn
	settingsPage string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchAppFromSettings,
		Desc: "Peripherals app can be found and launched from the settings",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val: settingsTestParams{
					appName:      apps.Diagnostics.Name,
					featureFlag:  "DiagnosticsApp",
					waitForApp:   diagnosticsapp.WaitForApp,
					settingsPage: "help", // URL for About ChromeOS page
				},
			},
		},
	})
}

// LaunchAppFromSettings verifies launching an app from the settings.
func LaunchAppFromSettings(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	params := s.Param().(settingsTestParams)

	cr, err := chrome.New(ctx, chrome.EnableFeatures(params.featureFlag))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx) // Close our own chrome instance

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	entryParams := ui.FindParams{
		Name: params.appName,
		Role: ui.RoleTypeLink,
	}

	// Condition to satisfy verification of LaunchAtPageURL
	condition := func(ctx context.Context) (bool, error) {
		return ossettings.DescendantNodeExists(ctx, tconn, entryParams)
	}
	if err := ossettings.LaunchAtPageURL(ctx, tconn, cr, params.settingsPage, condition); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	if err := ui.StableFindAndClick(ctx, tconn, entryParams, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}); err != nil {
		s.Fatal("Failed to find and click entry: ", err)
	}

	// App should be launched.
	if err := params.waitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
}
