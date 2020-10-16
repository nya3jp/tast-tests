// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package diagnostics

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/diagnosticsapp"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LaunchDiagnosticsFromLauncher,
		Desc: "Diagnostics app can be found and launched from the launcher",
		Contacts: []string{
			"joonbug@chromium.org",
			"cros-peripherals@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// LaunchDiagnosticsFromLauncher verifies launching diagnostics app from the launcher.
func LaunchDiagnosticsFromLauncher(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("DiagnosticsApp"))
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

	// Search for "diagnostic".
	if err := launcher.Search(ctx, tconn, "diagnostic"); err != nil {
		s.Fatal("Failed to search for diagnostics: ", err)
	}

	// Diagnostics app should be one of the search results.
	appNode, err := launcher.WaitForAppResult(ctx, tconn, "Diagnostics", 15*time.Second)
	if err != nil {
		s.Fatal("Diagnostics app does not exist in search result: ", err)
	}

	// Clicking that result should open the Diagnostics app.
	if err := appNode.LeftClick(ctx); err != nil {
		s.Fatal("Failed to launch app from search result: ", err)
	}

	// App should be launched.
	if err := diagnosticsapp.WaitForApp(ctx, tconn); err != nil {
		s.Fatal("Failed to launch diagnostics app: ", err)
	}
}
