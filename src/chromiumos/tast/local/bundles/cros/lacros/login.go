// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	// loginRestoreURL is the URL that is expected to be running in Lacros after the UI is restarted.
	loginRestoreURL = "https://abc.xyz"

	// loginRestoreURLtitle is the URL title used for testing. It has to match with loginRestoreURL.
	loginRestoreURLtitle = "Alphabet"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Login,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Measures Lacros login time after a full restore UI session",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Params: []testing.Param{{
			Fixture: "lacros",
		}, {
			Name:    "primary",
			Fixture: "lacrosPrimary",
		}},
	})
}

func Login(ctx context.Context, s *testing.State) {
	f := s.FixtValue()
	cr := f.(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	// Launch Lacros via shelf.
	l, err := lacros.LaunchFromShelf(ctx, tconn, s.FixtValue().(lacrosfixt.FixtValue).LacrosPath())
	if err != nil {
		s.Fatal("Failed to launch Lacros: ", err)
	}
	lacrosClosedByChrome := false
	defer func() {
		if !lacrosClosedByChrome {
			if err := l.Close(ctx); err != nil {
				s.Fatal("Failed to close Lacros: ", err)
			}
		}
	}()

	// Connect to Lacros and navigate to a URL.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err != nil {
		s.Fatalf("Failed to find new tab: %s", err)
	}
	defer conn.Close()

	if err := conn.Navigate(ctx, loginRestoreURL); err != nil {
		s.Fatalf("Failed to navigate to the URL: %s", err)
	}

	loginSetAlwaysRestoreSettings(ctx, tconn)

	// TODO(tvignatti): Explain reason why.
	lacrosfixt.ChromeUnlock()

	// Close session.
	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome: ", err)
	}

	lacrosClosedByChrome = true

	pv := perf.NewValues()

	s.Log("Start measuring login time")
	start := time.Now()

	// Restore previous session containing Lacros with the given URL.
	loginRestoredSession(ctx, s)

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
}

func loginRestoredSession(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		// Set not to clear the notification after restore.
		// By default, On startup is set to ask every time after reboot
		// and there is an alertdialog asking the user to select whether to restore or not.
		chrome.RemoveNotification(false),
		chrome.EnableFeatures("LacrosSupport", "FullRestore"),
		chrome.EnableRestoreTabs(),
		chrome.KeepState())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	chromeClosedByFixture := false
	defer func() {
		if !chromeClosedByFixture {
			if err := cr.Close(ctx); err != nil {
				s.Fatal("Failed to close Chrome: ", err)
			}
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Confirm that the lacros is restored.
	if err := lacros.WaitForLacrosWindow(ctx, tconn, loginRestoreURLtitle); err != nil {
		s.Fatal("Failed to restore Lacros: ", err)
	}

	// TODO(tvignatti): Explain reason why.
	lacrosfixt.ChromeLock()

	// TODO(tvignatti): Explain reason why.
	chromeClosedByFixture = true
}

func loginSetAlwaysRestoreSettings(ctx context.Context, tconn *chrome.TestConn) error {
	// Open OS settings to set the 'Always restore' setting.
	settings, err := ossettings.LaunchAtPage(ctx, tconn, nodewith.Name("Apps").Role(role.Link))
	if err != nil {
		return errors.Wrap(err, "failed to launch apps settings page")
	}

	if err := uiauto.Combine("set 'Always restore' Settings",
		uiauto.New(tconn).LeftClick(nodewith.Name("Restore apps on startup").Role(role.PopUpButton)),
		uiauto.New(tconn).LeftClick(nodewith.Name("Always restore").Role(role.ListBoxOption)))(ctx); err != nil {
		return errors.Wrap(err, "failed to set 'Always restore' Settings")
	}

	settings.Close(ctx)

	// According to the PRD of Full Restore go/chrome-os-full-restore-dd,
	// it uses a throttle of 2.5s to save the app launching and window statue information to the backend.
	// Therefore, sleep 3 seconds here.
	testing.Sleep(ctx, 3*time.Second)

	return nil
}
