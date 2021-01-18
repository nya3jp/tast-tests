// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUILauncher,
		Desc:         "Checks the behaviors of launching camera app via launcher",
		Contacts:     []string{"wtlee@google.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUILauncher(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	// Only tests clicking camera icon on launcher under clamshell mode since all apps will minimize when the launcher shows up in tablet mode.
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure in clamshell mode: ", err)
	}
	defer cleanup(ctx)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to launch camera app: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

	// If CCA is a platform app, when firing the launch event as the app is
	// currently showing, the app should minimize. But this behavior is not
	// implemented in SWA to make it consistent with other SWAs.
	if err := app.MinimizeWindow(ctx); err != nil {
		s.Fatal("Failed to minimize camera app: ", err)
	}
	if err := app.WaitForMinimized(ctx, true); err != nil {
		s.Fatal("Failed to wait for app being minimized: ", err)
	}

	// When firing the launch event as the app is minimized, the app window should be restored.
	if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
		s.Fatal("Failed to launch camera app: ", err)
	}
	if err := app.WaitForMinimized(ctx, false); err != nil {
		s.Fatal("Failed to wait for app being restored: ", err)
	}
}
