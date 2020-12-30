// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/chrome/ui/printmanagementapp"
	"chromiumos/tast/local/chrome/ui/scanapp"
	"chromiumos/tast/testing"
)

// testParams contains all the data needed to run a single test iteration.
type testParams struct {
	appName     string
	query       string
	featureFlag string
	waitForApp  peripheraltypes.WaitForAppFn
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchAppFromLauncher,
		Desc: "Peripherals app can be found and launched from the launcher",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
			"michaelcheco@google.com",
			"jschettler@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val: testParams{
					appName:     apps.Diagnostics.Name,
					query:       "diagnostic",
					featureFlag: "DiagnosticsApp",
					waitForApp:  diagnosticsapp.WaitForApp,
				},
			},
			{
				Name: "print_management",
				Val: testParams{
					appName:     apps.PrintManagement.Name,
					query:       apps.PrintManagement.Name,
					featureFlag: "",
					waitForApp:  printmanagementapp.WaitForApp,
				},
				Pre: chrome.LoggedIn(),
			},
			{
				Name: "scan",
				Val: testParams{
					appName:     apps.Scan.Name,
					query:       apps.Scan.Name,
					featureFlag: "ScanningUI",
					waitForApp:  scanapp.WaitForApp,
				},
			},
		},
	})
}

// LaunchAppFromLauncher verifies launching an app from the launcher.
func LaunchAppFromLauncher(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, ok := s.PreValue().(*chrome.Chrome) // Grab pre existing chrome instance
	if !ok {
		crWithFeature, err := chrome.New(ctx, chrome.EnableFeatures(s.Param().(testParams).featureFlag))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer crWithFeature.Close(cleanupCtx) // Close our own chrome instance
		cr = crWithFeature
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	err = launcher.SearchAndLaunchWithQuery(ctx, tconn, s.Param().(testParams).query, s.Param().(testParams).appName)
	if err != nil {
		s.Fatal("Failed to search and launch app: ", err)
	}

	// App should be launched.
	if err := s.Param().(testParams).waitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
}
