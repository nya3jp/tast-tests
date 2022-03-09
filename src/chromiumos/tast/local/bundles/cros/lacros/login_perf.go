// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const (
	// loginPerfRestoreURL is the URL used in Lacros at its first launch and then also the expected
	// one to be running in Lacros after the UI gets restarted.
	loginPerfRestoreURL = "https://abc.xyz"

	// loginPerfRestoreURLtitle is the URL title used for confirming in Lacros second launch, after
	// UI is restarted. It has to match with the content provided in loginPerfRestoreURL.
	loginPerfRestoreURLtitle = "Alphabet"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures Lacros login time after a full UI session restore",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{browserfixt.LacrosDeployedBinary},
		Params: []testing.Param{{
			Val:               browser.TypeLacros,
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name: "chrome",
			Val:  browser.TypeAsh,
		}},
	})
}

// LoginPerf measures the session time from login until the moment where the first browser window
// is shown. In this test, extra costs like mounting, launching process, disk checking, among
// others throughout the login session may provide improving possibilities for the developers.
func LoginPerf(ctx context.Context, s *testing.State) {
	bt := s.Param().(browser.Type)

	// Connect to a fresh ash-chrome instance to ensure the UI session first-run state, also get a
	// browser instance for using browser functionality.
	cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx, bt, browserfixt.DefaultLacrosConfig.WithVar(s))
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	// Connect to the browser and navigate to the URL that later, when collecting the performance
	// data is expected to be restored.
	conn, err := br.NewConn(ctx, loginPerfRestoreURL)
	if err != nil {
		s.Fatalf("Failed to connect to the %v restore URL: %v ", loginPerfRestoreURL, err)
	}
	defer conn.Close()

	// Open OS settings and sets the 'Always restore' setting.
	setAlwaysRestoreSettings(ctx, tconn)

	func() {
		pv := perf.NewValues()

		s.Log("Start measuring browser login time")
		start := time.Now()

		cr, err := chrome.New(ctx,
			// Set not to clear the notification after restore. By default, On startup is set to ask every time
			// after reboot and there is an alertdialog asking the user to select whether to restore or not.
			chrome.RemoveNotification(false),
			chrome.EnableFeatures("FullRestore"),
			// Disable whats-new page. See crbug.com/1271436.
			chrome.DisableFeatures("ChromeWhatsNewUI"),
			chrome.EnableRestoreTabs(),
			chrome.KeepState())
		if err != nil {
			s.Fatal("Failed to start ash-chrome: ", err)
		}
		defer cr.Close(ctx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to connect Test API: ", err)
		}

		defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

		// Confirm that the browser is restored.

		// TODO(tvignatti): waitForChromeWindow relies on ash.WaitForCondition which may delay the
		// return, resulting in misleading performance data. Check the comment below:

		// Polling often results in increased load and slower execution (since there's a delay between when something
		// happens and when the next polling cycle notices it). It should only be used as a last resort when there's no
		// other way to watch for an event. The preferred approach is to watch for events in select{} statements.
		// Goroutines can be used to provide notifications over channels.
		if err := waitForChromeWindow(ctx, bt, tconn, loginPerfRestoreURLtitle); err != nil {
			s.Fatalf("Failed to restore to %v browser: %v", bt, err)
		}

		loadTime := time.Since(start)
		s.Log("Stop measuring browser login time")

		pv.Set(perf.Metric{
			Name:      "browserRestore",
			Unit:      "seconds",
			Direction: perf.SmallerIsBetter,
		}, time.Duration(loadTime).Seconds())

		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed saving perf data: ", err)
		}
	}()
}

// setAlwaysRestoreSettings opens OS settings and sets the 'Always restore' setting. In order to
// avoid possible noise when collecting the browser login time performance at restoring time, this
// function also makes sure to close the OS settings app before returning.
func setAlwaysRestoreSettings(ctx context.Context, tconn *chrome.TestConn) error {
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

// waitForChromeWindow waits for a browser window to be open and have the title to be visible if it is specified as a param.
// TODO(tvignatti:) move this to wm.go, and replace all other WaitForLacrosWindow references.
func waitForChromeWindow(ctx context.Context, bt browser.Type, tconn *chrome.TestConn, title string) error {
	var topWindowName string
	switch bt {
	case browser.TypeAsh:
		topWindowName = "BrowserFrame"
	case browser.TypeLacros:
		topWindowName = "ExoShellSurface"
	default:
		return errors.Errorf("unrecognized browser type %s", string(bt))
	}

	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		if !w.IsVisible {
			return false
		}
		if !strings.HasPrefix(w.Name, topWindowName) {
			return false
		}
		if len(title) > 0 {
			return strings.Contains(w.Title, title)
		}
		return true
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for %v browser window to be visible (title: %v)", bt, title)
	}

	return nil
}
