// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: OpenTabSearch,
		Desc: "Test that Launcher search works with open tabs",
		Contacts: []string{
			"etuck@chromium.org",
			"tast-users@chromium.org",
		},
		Attr:    []string{"group:mainline", "informational"},
		Fixture: "chromeLoggedInWith100FakeAppsProductivityLauncher",
	})
}

func OpenTabSearch(ctx context.Context, s *testing.State) {
	// Reserve some time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer cr.Close(cleanupCtx)

	// For debugging purposes only.
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	// Open a new window.
	url := ""
	conn, err := cr.Browser().NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to open new window with url: %v, %v", url, err)
	}
	defer conn.Close()

	if err := launcher.OpenBubbleLauncher(tconn)(ctx); err != nil {
		s.Fatal("Failed to open bubble launcher: ", err)
	}
}
