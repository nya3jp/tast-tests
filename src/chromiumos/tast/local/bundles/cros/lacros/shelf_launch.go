// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShelfLaunch,
		Desc:         "Tests launching and interacting with lacros launched from the Shelf",
		Contacts:     []string{"lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Fixture:           "lacrosStartedByDataUI",
				ExtraData:         []string{launcher.DataArtifact},
				ExtraSoftwareDeps: []string{"lacros_stable"},
			},
			{
				Name:              "unstable",
				Fixture:           "lacrosStartedByDataUI",
				ExtraData:         []string{launcher.DataArtifact},
				ExtraSoftwareDeps: []string{"lacros_unstable"},
				ExtraAttr:         []string{"informational"},
			},
			{
				Name:      "rootfs",
				Fixture:   "lacrosStartedFromRootfs",
				ExtraAttr: []string{"informational"},
			},
			{
				Name:              "omaha",
				Fixture:           "lacrosStartedByOmaha",
				ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
				ExtraAttr:         []string{"informational"},
			}},
	})
}

func ShelfLaunch(ctx context.Context, s *testing.State) {
	f := s.FixtValue().(launcher.FixtData)
	if f.Mode == launcher.PreExist {
		// TODO(crbug.com/1127165): Remove this when we can use Data in fixtures.
		if err := launcher.EnsureLacrosChrome(ctx, f, s.DataPath(launcher.DataArtifact)); err != nil {
			s.Fatal("Failed to extract lacros binary: ", err)
		}
	}

	tconn, err := f.Chrome.TestAPIConn(ctx)
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
	if err := launcher.WaitForLacrosWindow(ctx, tconn, "New Tab"); err != nil {
		s.Fatal("Failed waiting for Lacros window to be visible: ", err)
	}

	s.Log("Connecting to the lacros-chrome browser")
	l, err := launcher.ConnectToLacrosChrome(ctx, f.LacrosPath, launcher.LacrosUserDataDir)
	if err != nil {
		s.Fatal("Failed to connect to lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	// s.Log("Opening a new tab")
	// tab, err := l.Devsess.CreateTarget(ctx, "about:blank")
	// if err != nil {
	// 	s.Fatal("Failed to open new tab: ", err)
	// }
	// defer l.Devsess.CloseTarget(ctx, tab)
	// if err := launcher.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
	// 	s.Fatal("Failed waiting for Lacros to navigate to about:blank page: ", err)
	// }

	// TODO(b/187795078): DO NOT SUBMIT see if UI element on web content inside Lacros is accessible via uiauto using the test extension from Ash.
	s.Log("a11y: starting")
	opts := testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: 10 * time.Second}
	ui := uiauto.New(tconn)
	restoreBtn := nodewith.ClassName("FrameCaptionButton").Name("Restore")
	closeBtn := nodewith.ClassName("FrameCaptionButton").Name("Close")
	testing.Sleep(ctx, 3*time.Second)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("a11y: search for a restore button on lacros window frame, then click it")
	if err := ui.WithPollOpts(opts).WaitUntilExists(restoreBtn)(ctx); err != nil {
		s.Fatal("Failed to find the button: ", err)
	}
	if err := ui.LeftClick(restoreBtn)(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	testing.Sleep(ctx, 3*time.Second)

	s.Log("a11y: search for an element ('Web Store' shortcut) on web content page inside lacros, then click it")
	webstoreBtn := nodewith.Role("staticText").Name("Web Store")
	if err := ui.LeftClick(webstoreBtn)(ctx); err != nil {
		s.Fatal("Failed to find the button inside Lacros: ", err)
	}
	testing.Sleep(ctx, 3*time.Second)

	s.Log("a11y: close lacros by clicking the close button")
	if err := ui.LeftClick(closeBtn)(ctx); err != nil {
		s.Fatal("Failed to click the button: ", err)
	}
	s.Log("a11y: lacros should be closed now")
	testing.Sleep(ctx, 3*time.Second)
	//~

	s.Log("Closing lacros-chrome browser")
	if err := l.Close(ctx); err != nil {
		s.Fatal("Failed to close lacros-chrome: ", err)
	}

	if err := ash.WaitForAppClosed(ctx, tconn, apps.Lacros.ID); err != nil {
		s.Fatalf("%s did not close successfully: %s", apps.Lacros.Name, err)
	}
}
