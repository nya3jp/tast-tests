// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	// loginPerfRestoreURL is the URL that is expected to be running in Lacros after the UI is restarted.
	loginPerfRestoreURL = "https://abc.xyz"

	// loginPerfRestoreURLtitle is the URL title used for testing. It has to match with loginPerfRestoreURL.
	loginPerfRestoreURLtitle = "Alphabet"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures Lacros login time after a full restore UI session",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Vars:         []string{browserfixt.LacrosDeployedBinary},
	})
}

func LoginPerf(ctx context.Context, s *testing.State) {
	func() {
		bt := browser.TypeLacros

		// Connect to a fresh Chrome instance to ensure holding space first-run state,
		// also get a browser instance for using browser functionality.
		cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx, bt, browserfixt.DefaultLacrosConfig.WithVar(s))
		if err != nil {
			s.Fatalf("Failed to connect to %v browser: %v", bt, err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect to test API: ", err)
		}

		// Connect to Lacros and navigate to the URL to be restored.
		conn, err := br.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
		if err != nil {
			s.Fatalf("Failed to find new tab: %s", err)
		}
		defer conn.Close()

		if err := conn.Navigate(ctx, loginPerfRestoreURL); err != nil {
			s.Fatalf("Failed to navigate to the URL: %s", err)
		}

		// Open OS settings to set the 'Always restore' setting.
		settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Link))
		if err != nil {
			s.Fatal("Failed to launch Apps Settings: ", err)
		}

		if err := uiauto.Combine("set 'Always restore' Settings",
			uiauto.New(tconn).LeftClick(nodewith.Name("Restore apps on startup").Role(role.PopUpButton)),
			uiauto.New(tconn).LeftClick(nodewith.Name("Always restore").Role(role.ListBoxOption)))(ctx); err != nil {
			s.Fatal("Failed to set 'Always restore' Settings: ", err)
		}

		settings.Close(ctx)

		// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
		// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
		// Therefore, sleep 3 seconds here.
		testing.Sleep(ctx, 3*time.Second)
	}()

	func() {
		pv := perf.NewValues()

		s.Log("Start measuring login time")
		start := time.Now()

		cr, err := chrome.New(ctx,
			// Set not to clear the notification after restore.
			// By default, On startup is set to ask every time after reboot
			// and there is an alertdialog asking the user to select whether to restore or not.
			chrome.RemoveNotification(false),
			chrome.EnableFeatures("FullRestore"),
			chrome.EnableRestoreTabs(),
			chrome.KeepState())
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}

		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		// Confirm that the lacros is restored.
		if err := lacros.WaitForLacrosWindow(ctx, tconn, loginPerfRestoreURLtitle); err != nil {
			s.Fatal("Failed to restore Lacros: ", err)
		}

		loadTime := time.Since(start)
		s.Log("Stop measuring login time")

		pv.Set(perf.Metric{
			Name:      "lacrosLogin",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, time.Duration(loadTime).Seconds())

		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}()
}
