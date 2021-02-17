// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/local/apps"
	applauncher "chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppLauncherLaunch,
		Desc:         "Tests launching lacros from the App Launcher",
		Contacts:     []string{"liaoyuke@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosStartedByDataUI",
		Data:         []string{launcher.DataArtifact},
	})
}

func AppLauncherLaunch(ctx context.Context, s *testing.State) {
	// TODO(crbug.com/1127165): Remove this when we can use Data in fixtures.
	f := s.FixtValue().(launcher.FixtData)
	if err := launcher.EnsureLacrosChrome(ctx, f, s.DataPath(launcher.DataArtifact)); err != nil {
		s.Fatal("Failed to extract lacros binary: ", err)
	}

	tconn, err := f.Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Clean up user data dir to ensure a clean start.
	os.RemoveAll(launcher.LacrosUserDataDir)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := applauncher.SearchAndLaunchWithQuery(tconn, kb, "lacros", apps.Lacros.Name)(ctx); err != nil {
		s.Fatal("Failed to search and launch Lacros app: ", err)
	}

	s.Log("Connecting to the lacros-chrome browser")
	p := s.FixtValue().(launcher.FixtData)
	l, err := launcher.ConnectToLacrosChrome(ctx, p.LacrosPath, launcher.LacrosUserDataDir)
	if err != nil {
		s.Fatal("Failed to connect to lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	s.Log("Opening a new tab")
	tab, err := l.Devsess.CreateTarget(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer l.Devsess.CloseTarget(ctx, tab)

	s.Log("Checking that Lacros window is visible")
	if err := launcher.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
		s.Fatal("Failed waiting for Lacros to navigate to about:blank page: ", err)
	}
}
