// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanning

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/chrome/ui/scanapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchScanAppFromLauncher,
		Desc: "Verifies the Scan app can be launched from the launcher",
		Contacts: []string{
			"jschettler@chromium.org",
			"cros-peripherals@google.com",
			"project-bolton@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchScanAppFromLauncher verifies the Scan app can be launched from the
// launcher.
func LaunchScanAppFromLauncher(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ScanningUI"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := launcher.OpenLauncher(ctx, tconn); err != nil {
		s.Fatal("Failed to open launcher: ", err)
	}

	if err := launcher.Search(ctx, tconn, apps.Scan.Name); err != nil {
		s.Fatal("Failed to search for Scan app: ", err)
	}

	appNode, err := launcher.WaitForAppResult(ctx, tconn, apps.Scan.Name, 10*time.Second)
	if err != nil {
		s.Fatal("Scan app does not exist in search result: ", err)
	}

	if err := appNode.LeftClick(ctx); err != nil {
		s.Fatal("Failed to launch Scan app from search result: ", err)
	}

	if err := scanapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch Scan app: ", err)
	}
}
