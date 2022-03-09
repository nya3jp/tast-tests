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
	"chromiumos/tast/testing/hwdep"
)

type testParameters struct {
	bt  browser.Type
	cfg *browserfixt.LacrosConfig
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartupPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Captures startup metrics for Lacros configurations and modes",
		Contacts:     []string{"hidehiko@chromium.org", "tvignatti@igalia.com", "lacros-team@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Name:              "rootfs_primary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: testParameters{
				browser.TypeLacros,
				browserfixt.DefaultLacrosConfig,
			},
		}, {
			Name:              "rootfs_sidebyside",
			ExtraSoftwareDeps: []string{"lacros"},
			Val: testParameters{
				browser.TypeLacros,
				browserfixt.NewLacrosConfig(lacrosfixt.Rootfs, lacrosfixt.LacrosSideBySide),
			},
		}, {
			Name:              "omaha_primary",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky", "phaser")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			Val: testParameters{
				browser.TypeLacros,
				browserfixt.NewLacrosConfig(lacrosfixt.Omaha, lacrosfixt.LacrosPrimary),
			},
		}, {
			Name:              "omaha_sidebyside",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky", "phaser")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			Val: testParameters{
				browser.TypeLacros,
				browserfixt.NewLacrosConfig(lacrosfixt.Omaha, lacrosfixt.LacrosSideBySide),
			},
		}, {
			Name: "chrome",
			Val: testParameters{
				browser.TypeAsh,
				browserfixt.DefaultLacrosConfig,
			},
		}},
		Vars: []string{browserfixt.LacrosDeployedBinary},
	})
}

type startupMetrics struct {
	// Time to restart UI and login the user. It is the time between when the UI gets restarted and
	// the user successfully logs in within Ash. Login time is recorded by the D-Bus signal time
	// emitted by session_manager, which in turn was just triggered by Ash.
	// This metric may use the Internet (e.g. when login is handled by Gaia), which is unreliable
	// and therefore needs to be carefully used when analysing the collected data.
	loginTime time.Duration

	// Time to restore the browser window. It is the time between when the UI gets restarted and
	// the browser (Lacros or Ash-Chrome) window is restored and visible to the user. This is
	// recorded by tast using its event polling mechanism.
	// Because this uses polling for checking when the window is visible, this "metric" is
	// unsuitable for proper measurements. We most likely will want to get rid of this as soon as
	// this work stabilizes and we have a good understanding of the startup analysis.
	windowRestoreTime time.Duration

	// Time to start the Lacros browser. It is the time between when the process is launched and
	// the mojo connection is established between Ash and Lacros. This is recorded by Ash (via
	// ChromeOS.Lacros.StartTime histogram metric) each time Lacros browser is started.
	lacrosStartTime time.Duration
}

// StartupPerf measures Chrome OS from session start until the moment where the first browser
// window is shown. There are few metrics collected at the moment such as session start time,
// browser restore time and others in order to provide optimization possibilities for the
// developers.
func StartupPerf(ctx context.Context, s *testing.State) {
	param := s.Param().(testParameters)

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

	const iterationCount = 7
	pv := perf.NewValues()
	for i := 0; i < iterationCount; i++ {
		testing.ContextLogf(ctx, "StartupPerf: Running iteration %d/%d", i+1, iterationCount)
		// Start UI, open a browser and leave a window opened in 'server.URL'.
		if err := performInitialLogin(ctx, param.bt, param.cfg.WithVar(s), s.OutDir(), s.HasError, server.URL); err != nil {
			s.Fatal("Failed to do initial login: ", err)
		}

		// Start to collect data, restart UI, wait for browser window to be opened and get the
		// metrics.
		v, err := performRegularLogin(ctx, param.bt, param.cfg.WithVar(s), s.OutDir(), s.HasError, sm, server.URL, htmlPageTitle)
		if err != nil {
			s.Error("Failed to do regular login: ", err)
			// keep on the next iteration in case of failure
			continue
		}

		// Record the metrics.
		recordStartupPerf(pv, "login_time", v.loginTime)
		recordStartupPerf(pv, "window_restore_time", v.windowRestoreTime)
		recordStartupPerf(pv, "lacros_start_time", v.lacrosStartTime)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
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

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, htmlPageWithSpecificTitle)
	}))
}

// performInitialLogin does the initial UI session login and opens a very particular browser
// (Lacros or Ash-Chrome) window. This function intentionally leaves this particular window opened,
// so the next UI session (see performRegularLogin) will restore it in order to record all the
// browser window start time.
func performInitialLogin(ctx context.Context, browserType browser.Type, cfg *browserfixt.LacrosConfig, outDir string, hasError func() bool, url string) error {
	// Connect to a fresh ash-chrome instance to ensure the UI session first-run state, and also
	// get a browser instance. We don't intentionally get a closeBrowser closure here in order to
	// restore the session later during performRegularLogin.
	//
	// TODO(tvignatti): this performs fake login, which is different from actual login flow, and
	// probably bypasses some key flow. That could be done if performed by Gaia login though. So to
	// avoid fake that, as this is performance test and we want to be more careful, we want to
	// change to use Gaia accounts instead.
	cr, br, _, err := browserfixt.SetUpWithNewChrome(ctx, browserType, cfg)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to %v browser", browserType)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	defer faillog.DumpUITreeOnError(ctx, outDir, hasError, tconn)

	// Connect to the browser and navigate to the URL that later, when collecting the performance
	// data, is expected to be restored.
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to the %v restore URL", url)
	}
	defer conn.Close()

	// Open OS settings and sets the 'Always restore' setting.
	setAlwaysRestoreSettings(ctx, tconn)

	return nil
}

// performRegularLogin restores browser (Lacros or Ash-Chrome) window and record start time.
func performRegularLogin(ctx context.Context, browserType browser.Type, cfg *browserfixt.LacrosConfig, outDir string, hasError func() bool, sm *session.SessionManager, url, expectedTitle string) (startupMetrics, error) {
	var v startupMetrics
	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		return v, errors.Wrap(err, "failed to wait until CPU is cooled down")
	}

	// Start listening for a "started" SessionStateChanged D-Bus signal from session_manager.
	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		return v, errors.Wrap(err, "failed to watch for D-Bus signals")
	}
	defer sw.Close(ctx)

	opts := []chrome.Option{
		chrome.EnableFeatures("FullRestore"),
		// Disable whats-new page. See crbug.com/1271436.
		chrome.DisableFeatures("ChromeWhatsNewUI"),
		chrome.EnableRestoreTabs(),
		chrome.KeepState()}

	if browserType == browser.TypeLacros {
		defaultOpts, err := lacrosfixt.DefaultOpts(cfg)
		if err != nil {
			return v, errors.Wrap(err, "failed to get default options")
		}
		opts = append(opts, defaultOpts...)
	}

	testing.ContextLog(ctx, "StartupPerf: Starting to collect metrics")
	startTime := time.Now()

	// Connect to new ash-chrome instance expecting that the browser (Lacros or Ash-Chrome) gets
	// fully restored.
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return v, errors.Wrap(err, "failed to start UI")
	}
	defer cr.Close(ctx)

	select {
	case <-sw.Signals:
		testing.ContextLog(ctx, "StartupPerf: Got SessionStateChanged \"started\" signal")
	case <-ctx.Done():
		return v, errors.Wrap(err, "didn't get SessionStateChanged signal")
	}
	v.loginTime = time.Since(startTime)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return v, errors.Wrap(err, "failed to connect Test API")
	}
	defer faillog.DumpUITreeOnError(ctx, outDir, hasError, tconn)

	// Confirm that the browser is restored.
	// TODO(tvignatti): waitForWindow relies on ash.WaitForCondition which is implemented using
	// event polling. Poll means a (high) possibility of delaying returns, thus making it an
	// unsuitable mechanism for collecting performance data. Therefore, we need a more accurate
	// way for getting browser notifications to confirm that the browser window is restored.
	if err := waitForWindow(ctx, browserType, tconn, expectedTitle); err != nil {
		return v, errors.Wrapf(err, "failed to restore to %v browser", browserType)
	}

	v.windowRestoreTime = time.Since(startTime)
	testing.ContextLog(ctx, "StartupPerf: Got browser restore time")

	if browserType == browser.TypeLacros {
		v.lacrosStartTime, err = histogramToDuration(ctx, tconn, "ChromeOS.Lacros.StartTime")
		if err != nil {
			return v, errors.Wrap(err, "failed to get histogram")
		}
		testing.ContextLog(ctx, "StartupPerf: Got ChromeOS.Lacros.StartTime")
	}

	return v, nil
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

	// TODO(crbug.com/1314785)
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
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for %v browser window to be visible (title: %v)", bt, title)
	}

	return nil
}

// histogramToDuration reads histogram and converts it to Duration.
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

// recordStartupPerf records login performance metric.
func recordStartupPerf(value *perf.Values, name string, duration time.Duration) {
	value.Append(perf.Metric{
		Name:      name,
		Unit:      "seconds",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, duration.Seconds())
}
