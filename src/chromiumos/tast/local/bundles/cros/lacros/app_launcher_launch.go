// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/lacros/launcher"
	applauncher "chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppLauncherLaunch,
		Desc:         "Tests launching lacros from the App Launcher",
		Contacts:     []string{"liaoyuke@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacrosUI",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}, {
			Name:              "unstable",
			ExtraSoftwareDeps: []string{"lacros_unstable"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func AppLauncherLaunch(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(launcher.FixtData)
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

	s.Log("Checking that Lacros window is visible")
	if err := launcher.WaitForLacrosWindow(ctx, tconn, "New Tab"); err != nil {
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	l, err := launcher.ConnectToLacrosChrome(ctx, f.LacrosPath, launcher.LacrosUserDataDir)
	if err != nil {
		s.Fatal("Failed to connect to lacros-chrome: ", err)
	}
	defer l.Close(ctx)
}
