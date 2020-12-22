// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/local/apps"
	uiLauncher "chromiumos/tast/local/chrome/ui/launcher"
	"chromiumos/tast/local/lacros/launcher"
	lacrosLauncher "chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppLauncherLaunch,
		Desc:         "Tests launching lacros from the App Launcher",
		Contacts:     []string{"liaoyuke@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Pre:          launcher.StartedByDataUI(),
		Data:         []string{lacrosLauncher.DataArtifact},
	})
}

func AppLauncherLaunch(ctx context.Context, s *testing.State) {
	tconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Clean up user data dir to ensure a clean start.
	os.RemoveAll(launcher.LacrosUserDataDir)
	if err := uiLauncher.SearchAndLaunchWithQuery(ctx, tconn, "lacros", apps.Lacros.Name); err != nil {
		s.Fatal("Failed to search and launch Lacros app: ", err)
	}

	s.Log("Checking that Lacros window is visible")
	if err := lacrosLauncher.WaitForLacrosWindow(ctx, tconn, "Welcome to Chrome"); err != nil {
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	l, err := launcher.ConnectToLacrosChrome(ctx, s.PreValue().(launcher.PreData).Chrome, launcher.LacrosUserDataDir)
	if err != nil {
		s.Fatal("Failed to connect to lacros-chrome: ", err)
	}
	defer l.Close(ctx)
}
