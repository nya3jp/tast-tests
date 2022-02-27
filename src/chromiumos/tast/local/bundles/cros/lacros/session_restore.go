// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SessionRestore,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests launching and interacting with lacros launched from the Shelf",
		Contacts:     []string{"lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{
			{
				Name:              "primary",
				Fixture:           "lacrosPrimaryRestorable",
				ExtraSoftwareDeps: []string{"lacros_stable"},
				ExtraAttr:         []string{"informational"},
			}},
	})
}

func SessionRestore(ctx context.Context, s *testing.State) {
	func() {
		f := s.FixtValue().(lacrosfixt.FixtValue)
		tconn, err := f.Chrome().TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to test API: ", err)
		}

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

		if err := lacros.WaitForLacrosWindow(ctx, tconn, "New Tab"); err != nil {
			// Grab Lacros logs to assist debugging before exiting.
			if errCopy := fsutil.CopyFile(filepath.Join(lacros.UserDataDir, "lacros.log"), filepath.Join(s.OutDir(), "lacros.log")); errCopy != nil {
			}
			s.Fatal("Failed waiting for Lacros window to be visible: ", err)
		}

		l, err := lacros.Connect(ctx, f.LacrosPath(), lacros.UserDataDir)
		if err != nil {
			s.Fatal("Failed to connect to lacros-chrome: ", err)
		}
		defer func() {
			if l != nil {
				l.Close(ctx)
			}
		}()

		conn, err := l.NewConn(ctx, "about:blank")
		if err != nil {
			s.Fatal("Failed to open new tab: ", err)
			conn.Close()
			conn.CloseTarget(ctx)
		}

		if err := lacros.WaitForLacrosWindow(ctx, tconn, "about:blank"); err != nil {
			s.Fatal("Failed waiting for Lacros to navigate to about:blank page: ", err)
		}

		// Open OS settings to set the 'Always restore' setting.
		if _, err = ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Link)); err != nil {
			s.Fatal("Failed to launch Apps Settings: ", err)
		}

		if err := uiauto.Combine("set 'Always restore' Settings",
			uiauto.New(tconn).LeftClick(nodewith.Name("Restore apps on startup").Role(role.PopUpButton)),
			uiauto.New(tconn).LeftClick(nodewith.Name("Always restore").Role(role.ListBoxOption)))(ctx); err != nil {
			s.Fatal("Failed to set 'Always restore' Settings: ", err)
		}

		// TODO : restart-ui session here

		// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
		// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
		// Therefore, sleep 3 seconds here.
		testing.Sleep(ctx, 3*time.Second)
	}()

	func() {
		// TODO : code after restart-ui
	}()
}
