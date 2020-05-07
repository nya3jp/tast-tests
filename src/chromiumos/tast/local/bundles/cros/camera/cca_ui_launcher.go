// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
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

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	app, err := cca.Init(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), func(tconn *chrome.TestConn) error {
		if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.Fatal("Failed to launch camera app: ", err)
	}

	// Only tests clicking camera icon on launcher under clamshell mode since all apps will minimize when the launcher shows up in tablet mode.
	tabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to check if it is tablet mode: ", err)
	}
	if !tabletMode {
		// When firing the launch event as the app is currently showing, the app should minimize.
		if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
			s.Fatal("Failed to launch camera app: ", err)
		}
		if isMinimized, err := app.IsWindowMinimized(ctx); err != nil {
			s.Fatal("Failed to check if window is minimized: ", err)
		} else if !isMinimized {
			s.Error("App should be minimized after firing launch when the window is shown")
		}

		// When firing the launch event as the app is minimized, the app window should be restored.
		if err := launcher.SearchAndLaunch(ctx, tconn, "Camera"); err != nil {
			s.Fatal("Failed to launch camera app: ", err)
		}
		if isMinimized, err := app.IsWindowMinimized(ctx); err != nil {
			s.Fatal("Failed to check if window is minimized: ", err)
		} else if isMinimized {
			s.Error("App should be restored after firing launch when the window is minimized")
		}
	}
}
