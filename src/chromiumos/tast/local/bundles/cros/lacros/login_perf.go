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
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

const (
	// loginPerfRestoreURL is the URL used in the browser's first launch and also the expected one
	// to be running after the UI gets restarted.
	loginPerfRestoreURL = "https://abc.xyz"

	// loginPerfRestoreURLtitle is the URL title used for confirming the second launch, after UI is
	// restarted. It has to match with the content provided in loginPerfRestoreURL.
	loginPerfRestoreURLtitle = "Alphabet"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Captures login metrics for Lacros",
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

// LoginPerf measures Chrome OS login from session start until the moment where the first browser
// window is shown. Metrics such as session start, mounting, launching process, disk checking,
// among others concerning login are captured in order to provide optimization possibilities for
// the developers.
func LoginPerf(ctx context.Context, s *testing.State) {
	bt := s.Param().(browser.Type)

	// Connect to a fresh ash-chrome instance to ensure the UI session first-run state, also get a
	// browser instance.
	cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx, bt, browserfixt.DefaultLacrosConfig.WithVar(s))
	if err != nil {
		s.Fatalf("Failed to connect to %v browser: %v", bt, err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Connect to the browser and navigate to the URL that later, when collecting the performance
	// data, is expected to be restored.
	conn, err := br.NewConn(ctx, loginPerfRestoreURL)
	if err != nil {
		s.Fatalf("Failed to connect to the %v restore URL: %v ", loginPerfRestoreURL, err)
	}
	defer conn.Close()

	// Open OS settings and sets the 'Always restore' setting.
	setAlwaysRestoreSettings(ctx, tconn)

	// Connects to session_manager via D-Bus.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect session_manager: ", err)
	}

	getLoginMetrics(ctx, s, bt, sm)
}

func getLoginMetrics(ctx context.Context, s *testing.State, bt browser.Type, sm *session.SessionManager) error {
	// TODO(tvignatti): WaitUntilCoolDown?

	pv := perf.NewValues()

	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	testing.ContextLog(ctx, "LoginPerf: Starting to collect metrics")
	ashChromeStartTime := time.Now()

	// Connect to new ash-chrome instance expecting that the browser gets restored.
	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("FullRestore"),
		// Disable whats-new page. See crbug.com/1271436.
		chrome.DisableFeatures("ChromeWhatsNewUI"),
		chrome.EnableRestoreTabs(),
		chrome.KeepState())
	if err != nil {
		return errors.Wrap(err, "failed to start ash-chrome")
	}
	defer cr.Close(ctx)

	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "LoginPerf: Got SessionStateChanged \"started\" signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}
	sessionStartTime := time.Since(ashChromeStartTime)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect Test API")
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Confirm that the browser is restored.
	// TODO(tvignatti): waitForWindow relies on ash.WaitForCondition which is implemented using
	// event polling. Poll means a (high) possibility of delaying returns, thus making it an
	// unsuitable mechanism for collecting performance data. Therefore, we need a more accurate
	// way for getting browser notifications when needing to confirm that the browser window is
	// restored.
	if err := waitForWindow(ctx, bt, tconn, loginPerfRestoreURLtitle); err != nil {
		return errors.Wrapf(err, "failed to restore to %v browser", bt)
	}

	browserRestoreTime := time.Since(ashChromeStartTime)
	testing.ContextLog(ctx, "LoginPerf: Got browser restore time")

	// TODO(tvignatti): Extract more metrics from chrome log
	// chromeReader, err := syslog.NewReader(ctx, syslog.SourcePath(cr.LogFilename()))
	// if err != nil {
	// 	s.Fatal("Failed to start log reader: ", err)
	// }
	// defer chromeReader.Close()

	// const expectedLogMsg = "Launching lacros-chrome at"
	// testing.ContextLog(ctx, "Waiting for lacros log message")
	// if _, err := chromeReader.Wait(ctx, 60*time.Second,
	// 	func(e *syslog.Entry) bool {
	// 		return strings.Contains(e.Content, expectedLogMsg)
	// 	}); err != nil {
	// 	s.Fatalf("Failed to wait for log msg \"%q\": %v", expectedLogMsg, err)
	// }

	pv.Set(perf.Metric{
		Name:      "sessionStart",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, time.Duration(sessionStartTime).Seconds())

	pv.Set(perf.Metric{
		Name:      "browserRestore",
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
	}, time.Duration(browserRestoreTime).Seconds())

	if err := pv.Save(s.OutDir()); err != nil {
		return errors.Wrap(err, "failed saving perf data")
	}

	return nil
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

	// According to the PRD of Full Restore go/chrome-os-full-restore-dd, it uses a throttle of
	// 2.5s to save the app launching and window statue information to the backend. Therefore,
	// sleep 3 seconds here.
	testing.Sleep(ctx, 3*time.Second)

	return nil
}

// waitForWindow waits for a browser window to be open and have the title to be visible if it is
// specified as a param.
// TODO(tvignatti): Move this to browserfixt.go (and possibly replace all WaitForLacrosWindow
// references with that).
func waitForWindow(ctx context.Context, bt browser.Type, tconn *chrome.TestConn, title string) error {
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
