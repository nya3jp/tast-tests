// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	applauncher "chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppLauncherLaunch,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests launching lacros from the App Launcher",
		Contacts:     []string{"liaoyuke@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
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
	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Clean up user data dir to ensure a clean start.
	os.RemoveAll(lacros.UserDataDir)
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()
	if err := applauncher.SearchAndLaunchWithQuery(tconn, kb, "lacros", apps.Lacros.Name)(ctx); err != nil {
		s.Fatal("Failed to search and launch Lacros app: ", err)
	}

	s.Log("Checking that Lacros window is visible")
	if err := lacros.WaitForLacrosWindow(ctx, tconn, "New Tab"); err != nil {
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	l, err := lacros.Connect(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to connect to lacros-chrome: ", err)
	}
	defer l.Close(ctx)
}
