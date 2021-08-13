// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// settingsTestParams contains all the data needed to run a single test iteration.
type settingsTestParams struct {
	appID        string
	menuLabel    string
	featureFlag  string
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{
			{
				Name: "diagnostics",
				Val: settingsTestParams{
					appID:        apps.Diagnostics.ID,
					menuLabel:    apps.Diagnostics.Name,
					featureFlag:  "DiagnosticsApp",
					settingsPage: "help", // URL for About ChromeOS page
				},
			},
			{
				Name: "scan",
				Val: settingsTestParams{
					appID:        apps.Scan.ID,
					menuLabel:    apps.Scan.Name + " Scan documents and images",
					settingsPage: "osPrinting", // URL for Print and scan page
				},
				ExtraAttr: []string{"group:paper-io", "paper-io_scanning"},
			},
			{
				Name: "print_management",
				Val: settingsTestParams{
					appID:        apps.PrintManagement.ID,
					menuLabel:    apps.PrintManagement.Name + " View and manage print jobs",
					settingsPage: "osPrinting", // URL for Print and scan page
				},
			},
		},
	})
}

// LaunchAppFromSettings verifies launching an app from the settings.
func LaunchAppFromSettings(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(settingsTestParams)

	cr, err := chrome.New(ctx, chrome.EnableFeatures(params.featureFlag))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx) // Close our own chrome instance.

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	ui := uiauto.New(tconn)
	entryFinder := nodewith.Name(params.menuLabel).Role(role.Link).Ancestor(ossettings.WindowFinder)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, params.settingsPage, ui.Exists(entryFinder)); err != nil {
		s.Fatal("Failed to launch Settings page: ", err)
	}

	if err := ui.LeftClick(entryFinder)(ctx); err != nil {
		s.Fatal("Failed to click entry: ", err)
	}

	err = ash.WaitForApp(ctx, tconn, params.appID, time.Minute)
	if err != nil {
		s.Fatal("Could not find app in shelf after launch: ", err)
	}
}
