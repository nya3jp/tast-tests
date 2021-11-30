// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfLaunch,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests launching and interacting with lacros launched from the Shelf",
		Contacts:     []string{"lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Fixture:           "lacrosUI",
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
			{
				Name:              "unstable",
				Fixture:           "lacrosUI",
				ExtraSoftwareDeps: []string{"lacros_unstable"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "primary",
				Fixture:           "lacrosPrimary",
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "primary_unstable",
				Fixture:           "lacrosPrimary",
				ExtraSoftwareDeps: []string{"lacros_unstable"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:              "omaha",
				Fixture:           "lacrosOmaha",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
				ExtraAttr:         []string{"informational"},
			}},
	})
}

func ShelfLaunch(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(lacrosfixt.FixtValue)
	tconn, err := f.Chrome().TestAPIConn(ctx)
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
		if appItem.AppID == apps.Lacros.ID && appItem.Name == apps.Lacros.Name && appItem.Type == ash.StandaloneBrowser {
			found = true
			break
		}
	}
	if !found {
		s.Logf("AppID: %v, Name: %v, Type: %v, was expected, but got",
			apps.Lacros.ID, apps.Lacros.Name, ash.StandaloneBrowser)
		for _, appItem := range appItems {
			s.Logf("AppID: %v, Name: %v, Type: %v", appItem.AppID, appItem.Name, appItem.Type)
		}
		s.Fatal("Lacros was not included in the list of installed applications: ", err)
	}

	s.Log("Checking that Lacros is a pinned app in the shelf")
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
	os.RemoveAll(lacros.UserDataDir)
	if err = ash.LaunchAppFromShelf(ctx, tconn, apps.Lacros.Name, apps.Lacros.ID); err != nil {
		s.Fatal("Failed to launch Lacros: ", err)
	}

	s.Log("Checking that Lacros window is visible")
	if err := lacros.WaitForLacrosWindow(ctx, tconn, "New Tab"); err != nil {
		// Grab Lacros logs to assist debugging before exiting.
		lacrosfaillog.Save(ctx, s.FixtValue().(lacrosfixt.FixtValue).LacrosPath())
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	s.Log("Connecting to the lacros-chrome browser")
	l, err := lacros.Connect(ctx, f.LacrosPath(), lacros.UserDataDir)
	if err != nil {
		s.Fatal("Failed to connect to lacros-chrome: ", err)
	}
	defer func() {
		if l != nil {
			l.Close(ctx)
		}
	}()

	s.Log("Opening a new tab")
	conn, err := l.NewConn(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)
	if err := lacros.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
		s.Fatal("Failed waiting for Lacros to navigate to about:blank page: ", err)
	}

	s.Log("Closing lacros-chrome browser")
	if err := l.Close(ctx); err != nil {
		s.Fatal("Failed to close lacros-chrome: ", err)
	}
	l = nil

	if err := ash.WaitForAppClosed(ctx, tconn, apps.Lacros.ID); err != nil {
		s.Fatalf("%s did not close successfully: %s", apps.Lacros.Name, err)
	}
}
