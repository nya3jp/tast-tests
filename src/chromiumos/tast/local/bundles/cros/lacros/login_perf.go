// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

type testParam struct {
	lacrosConfig lacrosfixt.LacrosConfig
	browserType  browser.Type
}

var lacrosSideBySideConfig = lacrosfixt.LacrosConfig{
	SetupMode: lacrosfixt.Rootfs,
	// TODO(tvignatti): lacrosfixt.NotSpecified is a misleading name but it's okay to use it for
	// now:
	// https://chromium-review.googlesource.com/c/chromiumos/platform/tast-tests/+/3499547/comment/9f82df17_60465e81
	LacrosMode: lacrosfixt.NotSpecified,
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Captures login metrics for Lacros",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{browserfixt.LacrosDeployedBinary},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros"},
			Val: testParam{
				lacrosConfig: lacrosSideBySideConfig,
				browserType:  browser.TypeLacros,
			},
		}, {
			Name:              "lacros_primary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: testParam{
				lacrosConfig: browserfixt.DefaultLacrosConfig,
				browserType:  browser.TypeLacros,
			},
		}, {
			Name: "chrome",
			Val: testParam{
				browserType: browser.TypeAsh,
			},
		}},
	})
}

// LoginPerf measures Chrome OS login from session start until the moment where the first browser
// window is shown. There are few metrics collected at the moment such as session start time,
// browser restore time and others in order to provide optimization possibilities for the
// developers.
func LoginPerf(ctx context.Context, s *testing.State) {
	var bt = s.Param().(testParam).browserType
	// This is the page title used to connect in the browser, right after the initial login, is
	// performed. This same title will be used later, during the regular login, to test if browser
	// restore worked, after the UI is restarted.
	const htmlPageTitle = `Hooray, this is a title!`
	server := createHTMLServer(htmlPageTitle)
	defer server.Close()

	// Connects to session_manager via D-Bus.
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect session_manager: ", err)
	}

	const iterationCount = 1
	pv := perf.NewValues()
	for i := 0; i < iterationCount; i++ {
		testing.ContextLogf(ctx, "LoginPerf: Running iteration %d/%d", i+1, iterationCount)
		// Start UI, open a browser and leave a window opened in 'server.URL'.
		err := performInitialLogin(ctx, s, bt, server.URL)
		if err != nil {
			s.Fatal("Failed to do initial login: ", err)
		}

		// Start to collect data, restart UI, wait for browser window to be opened and get the
		// metrics.
		sessionStartTime, browserRestoreTime, histogramLacrosStartTime, err := performRegularLogin(ctx, s, bt, sm, server.URL, htmlPageTitle)
		if err != nil {
			s.Fatal("Failed to do regular login: ", err)
		}

		// Record the metrics.
		recordLoginPerf(pv, "session_start", sessionStartTime)
		recordLoginPerf(pv, "browser_restore", browserRestoreTime)
		if bt == browser.TypeLacros {
			recordLoginPerf(pv, "histogram.lacros_start", histogramLacrosStartTime)
		}
		if err := pv.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
	}
}

// createHTMLServer creates and return a server that hosts a html page with a specified title.
func createHTMLServer(title string) *httptest.Server {
	htmlPageWithSpecificTitle := fmt.Sprintf(`<!doctype html>
	<html lang="en">
	<head>
	  <meta charset="utf-8">
	  <title>%v</title>
	</head>
	<body>
	</html>
	`, title)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, htmlPageWithSpecificTitle)
	}))
	return server
}

// performInitialLogin does the initial UI session login and opens a very particular browser
// (Lacros or Ash-Chrome) window. This funcion intentionally leaves this particular window opened,
// so the next UI session (see performRegularLogin) will restore it in order to record all the
// browser window start time.
func performInitialLogin(ctx context.Context, s *testing.State, bt browser.Type, url string) error {
	var cfg browserfixt.LacrosConfig
	if bt == browser.TypeLacros {
		cfg = s.Param().(testParam).lacrosConfig.WithVar(s)
	}

	// Connect to a fresh ash-chrome instance to ensure the UI session first-run state, and also
	// get a browser instance. We don't get a closeBrowser closure here so the session restore
	// happens as it should later during performRegularLogin.
	cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx, bt, cfg)
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
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		s.Fatalf("Failed to connect to the %v restore URL: %v ", url, err)
	}
	defer conn.Close()

	// Open OS settings and sets the 'Always restore' setting.
	setAlwaysRestoreSettings(ctx, tconn)

	return nil
}

// performRegularLogin TODO(tvignatti)
func performRegularLogin(ctx context.Context, s *testing.State, bt browser.Type, sm *session.SessionManager, url, expectedTitle string) (time.Duration, time.Duration, time.Duration, error) {
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to wait until CPU is cooled down")
	}

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
		return 0, 0, 0, errors.Wrap(err, "failed to start ash-chrome")
	}
	defer cr.Close(ctx)

	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "LoginPerf: Got SessionStateChanged \"started\" signal")
	case <-ctx.Done():
		return 0, 0, 0, errors.Wrap(err, "didn't get SessionStateChanged signal")
	}
	sessionStartTime := time.Since(ashChromeStartTime)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to connect Test API")
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Confirm that the browser is restored.
	// TODO(tvignatti): waitForWindow relies on ash.WaitForCondition which is implemented using
	// event polling. Poll means a (high) possibility of delaying returns, thus making it an
	// unsuitable mechanism for collecting performance data. Therefore, we need a more accurate
	// way for getting browser notifications to confirm that the browser window is restored.
	if err := waitForWindow(ctx, bt, tconn, expectedTitle); err != nil {
		return 0, 0, 0, errors.Wrapf(err, "failed to restore to %v browser", bt)
	}

	browserRestoreTime := time.Since(ashChromeStartTime)
	testing.ContextLog(ctx, "LoginPerf: Got browser restore time")

	if bt == browser.TypeAsh {
		return time.Duration(sessionStartTime), time.Duration(browserRestoreTime), 0, nil
	}

	// TODO(tvignatti): Extract more histogram Lacros-related metrics
	histogramLacrosStartTime, err := histogramToDuration(ctx, tconn, "ChromeOS.Lacros.StartTime")
	if err != nil {
		s.Fatal("Failed to get histogram: ", err)
	}
	testing.ContextLog(ctx, "LoginPerf: Got ChromeOS.Lacros.StartTime")

	return time.Duration(sessionStartTime), time.Duration(browserRestoreTime), histogramLacrosStartTime, nil
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

// histogramToDuration reads histogram and converts it to Duration. TODO(tvignatti): This is pretty
// much the same as readFirstAppLaunchHistogram, so find a better common place for it.
func histogramToDuration(ctx context.Context, tconn *chrome.TestConn, name string) (time.Duration, error) {
	metric, err := metrics.WaitForHistogram(ctx, tconn, name, 20*time.Second)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get %s histogram", name)
	}

	timeMs, err := metric.Mean()
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %s histogram", name)
	}

	return time.Duration(timeMs * float64(time.Millisecond)), nil
}

// recordLoginPerf TODO(tvignatti)
func recordLoginPerf(value *perf.Values, name string, duration time.Duration) {
	value.Append(perf.Metric{
		Name:      name,
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, duration.Seconds())
}
