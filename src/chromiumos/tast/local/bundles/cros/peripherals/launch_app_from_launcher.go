// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package peripherals

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// testParams contains all the data needed to run a single test iteration.
type testParams struct {
	app         apps.App
	query       string
	featureFlag string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchAppFromLauncher,
		Desc: "Peripherals app can be found and launched from the launcher",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
			"michaelcheco@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val: testParams{
					app:         apps.Diagnostics,
					query:       "diagnostic",
					featureFlag: "DiagnosticsApp",
				},
			},
			{
				Name: "print_management",
				Val: testParams{
					app:         apps.PrintManagement,
					query:       apps.PrintManagement.Name,
					featureFlag: "",
				},
				Pre: chrome.LoggedIn(),
			},
			{
				Name: "scan",
				Val: testParams{
					app:   apps.Scan,
					query: apps.Scan.Name,
				},
				ExtraAttr: []string{"group:paper-io", "paper-io_scanning"},
			},
		},
	})
}

// LaunchAppFromLauncher verifies launching an app from the launcher.
func LaunchAppFromLauncher(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
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

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	if err := launcher.SearchAndLaunchWithQuery(tconn, kb, s.Param().(testParams).query, s.Param().(testParams).app.Name)(ctx); err != nil {
		s.Fatal("Failed to search and launch app: ", err)
	}

	err = ash.WaitForApp(ctx, tconn, s.Param().(testParams).app.ID, time.Minute)
	if err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}
}
