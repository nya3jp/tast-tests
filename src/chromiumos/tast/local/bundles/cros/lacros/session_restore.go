// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
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
		Desc:         "Measure a Session-Restore timing for a lacros browser",
		Contacts:     []string{"lacros-team@google.com", "abhijeet@igalia.com"},
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
	f := s.FixtValue().(lacrosfixt.FixtValue)
	func() {
		cr := f.Chrome()
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Log("Failed to connect to the test API connection")
		}

		// Close all open windows before launching lacros-chrome
		ws, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get all open windows: ", err)
		}
		for _, w := range ws {
			if err := w.CloseWindow(ctx, tconn); err != nil {
				s.Logf("Warning: Failed to close window (%+v): %v", w, err)
			}
		}

		// Launch a lacros-chrome from shelf
		l, err := lacros.LaunchFromShelf(ctx, tconn, f.LacrosPath())
		if err != nil {
			s.Log("Failed to launch lacros")
		}

		conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
		if err != nil {
			s.Log("Failed to find new tab")
		}

		if err := conn.Navigate(ctx, chrome.BlankURL); err != nil {
			s.Log("Failed to navigate to the URL")
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

		// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
		// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
		// Therefore, sleep 3 seconds here.
		testing.Sleep(ctx, 3*time.Second)
	}()

	func() {
		// Get options used for launching a CrOS-chrome.
		options := f.Options()

		// TODO: Before restarting ui get the timestamp here.
		// Restart ui session.
		cr, err := chrome.New(ctx,
			options...)
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		// TODO : Check if lacros-browser is restored and get the timestamp here.
	}()

}
