// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfLaunch,
		Desc:         "Tests launching and interacting with lacros launched from the Shelf",
		Contacts:     []string{"lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Vars:         []string{"lacrosDeployedBinary"}, // Isn't applicable to omaha. TODO: stop applying to omaha once switched to fixture.
		Params: []testing.Param{
			{
				Pre:       launcher.StartedByDataUI(),
				ExtraData: []string{launcher.DataArtifact},
			},
			{
				Name:              "omaha",
				Pre:               launcher.StartedByOmaha(),
				ExtraHardwareDeps: hwdep.D(hwdep.Model("enguarde", "samus", "sparky")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			}},
	})
}

func ShelfLaunch(ctx context.Context, s *testing.State) {
	tconn, err := s.PreValue().(launcher.PreData).Chrome.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	s.Log("Checking that Lacros is included in installed apps")
	appItems, err := ash.ChromeApps(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get installed apps: ", err)
	}
	found := false
	for _, appItem := range appItems {
		if appItem.AppID == apps.Lacros.ID && appItem.Name == apps.Lacros.Name && appItem.Type == ash.Lacros {
			found = true
			break
		}
	}
	if !found {
		s.Fatal("Lacros was not included in the list of installed applications: ", err)
	}

	s.Log("Check that Lacros is a pinned app in the shelf")
	shelfItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get shelf items: ", err)
	}
	found = false
	for _, shelfItem := range shelfItems {
		if shelfItem.AppID == apps.Lacros.ID && shelfItem.Title == apps.Lacros.Name && shelfItem.Type == ash.ShelfItemTypePinnedApp {
			found = true
			break
		}
	}
	if !found {
		s.Fatal("Lacros was not found in the list of shelf items: ", err)
	}

	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Logf("Warning: Failed to close window (%+v): %v", w, err)
		}
	}

	// Clean up user data dir to ensure a clean start.
	os.RemoveAll(launcher.LacrosUserDataDir)
	if err = ash.LaunchAppFromShelf(ctx, tconn, apps.Lacros.Name, apps.Lacros.ID); err != nil {
		s.Fatal("Failed to launch Lacros: ", err)
	}

	s.Log("Checking that Lacros window is visible")
	if err := launcher.WaitForLacrosWindow(ctx, tconn, "Welcome to Chrome"); err != nil {
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	s.Log("Connecting to the lacros-chrome browser")
	p := s.PreValue().(launcher.PreData)
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
	s.Log("Closing lacros-chrome browser")

	if err := launcher.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
		s.Fatal("Failed waiting for Lacros to navigate to about:blank page: ", err)
	}

	if err := l.Close(ctx); err != nil {
		s.Fatal("Failed to close lacros-chrome: ", err)
	}

	if err := ash.WaitForAppClosed(ctx, tconn, apps.Lacros.ID); err != nil {
		s.Fatalf("%s did not close successfully: %s", apps.Lacros.Name, err)
	}
}
