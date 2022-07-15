// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/mafredri/cdp/rpcc"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParameters struct {
	bt              browser.Type     // browser.{TypeAsh/TypeLacros}
	lacrosSelection lacros.Selection // lacros.{Omaha,Rootfs}
	lacrosMode      lacros.Mode      // lacros.{LacrosPrimary,LacrosSideBySide}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LoginPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Measures performance and UI smoothness of Chrome OS login",
		Contacts: []string{
			"alemate@google.com",
			"oshima@google.com",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.gaiaPoolDefault",
		},
		// Test runs login / chrome restart 120+ times.
		Timeout: 120 * time.Minute,
		Data:    []string{"animation.html", "animation.js"},
		Params: []testing.Param{{
			Name: "ash_chrome",
			Val: testParameters{
				browser.TypeAsh, lacros.NotSelected, lacros.NotSpecified,
			},
		}, {
			Name: "lacros_chrome_root_fs_primary",
			Val: testParameters{
				browser.TypeLacros, lacros.Rootfs, lacros.LacrosPrimary,
			},
		}, {
			Name: "lacros_chrome_root_fs_side_by_side",
			Val: testParameters{
				browser.TypeLacros, lacros.Rootfs, lacros.LacrosSideBySide,
			},
		}, {
			Name:              "lacros_chrome_omaha_primary",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky", "phaser")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			Val: testParameters{
				browser.TypeLacros, lacros.Omaha, lacros.LacrosPrimary,
			},
		}, {
			Name:              "lacros_chrome_omaha_side_by_side",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("kled", "enguarde", "samus", "sparky", "phaser")), // Only run on a subset of devices since it downloads from omaha and it will not use our lab's caching mechanisms. We don't want to overload our lab.
			Val: testParameters{
				browser.TypeLacros, lacros.Omaha, lacros.LacrosSideBySide,
			},
		}},
	})
}

// loginPerfStartToLoginScreen starts Chrome to the login screen.
func loginPerfStartToLoginScreen(ctx context.Context, s *testing.State, browserType browser.Type, lacrosConfig *lacrosfixt.Config, arcOpt []chrome.Option, useTabletMode bool) (cr *chrome.Chrome, retErr error) {
	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
	options := []chrome.Option{
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.EnableFeatures("FullRestore"),
		chrome.EnableRestoreTabs(),
		chrome.SkipForceOnlineSignInForTesting(),
		chrome.EnableWebAppInstall(),
		chrome.HideCrashRestoreBubble(), // Ignore possible incomplete shutdown.
		// Disable whats-new page. See crbug.com/1271436.
		chrome.DisableFeatures("ChromeWhatsNewUI"),
	}
	if browserType == browser.TypeLacros {
		defaultOpts, err := lacrosConfig.Opts()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get default options")
		}
		options = append(options, defaultOpts...)
	} else {
		// For Ash we need to force session restore in another way.
		options = append(options, chrome.ForceLaunchBrowser())
	}
	cr, err := chrome.New(
		ctx,
		append(options, arcOpt...)...,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}
	defer func() {
		if retErr != nil {
			cr.Close(ctx)
		}
	}()

	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating login test api connection failed")
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), func() bool { return retErr != nil }, tLoginConn)

	if err = ash.SetTabletModeEnabled(ctx, tLoginConn, useTabletMode); err != nil {
		return nil, errors.Wrapf(err, "failed to set tablet mode %v", useTabletMode)
	}

	// Wait for the login screen to be ready for password entry.
	if st, err := lockscreen.WaitState(
		ctx,
		tLoginConn,
		func(st lockscreen.State) bool { return st.ReadyForPassword },
		30*time.Second); err != nil {
		return nil, errors.Wrapf(err, "failed waiting for the login screen to be ready for password entry: last state: %+v", st)
	}

	return cr, nil
}

// loginPerfDoLogin logs in and waits for animations to finish.
func loginPerfDoLogin(ctx context.Context, cr *chrome.Chrome, credentials chrome.Creds, browserType browser.Type) (l *lacros.Lacros, retErr error) {
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("no output directory exists")
	}
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating login test API connection failed")
	}
	defer faillog.DumpUITreeOnError(ctx, outdir, func() bool { return retErr != nil }, tLoginConn)

	// TODO(crbug/1109381): the password field isn't actually ready just yet when WaitState returns.
	// This causes it to miss some of the keyboard input, so the password will be wrong.
	// We can check in the UI for the password field to exist, which seems to be a good enough indicator that
	// the field is ready for keyboard input.
	if err := lockscreen.WaitForPasswordField(ctx, tLoginConn, credentials.User, 15*time.Second); err != nil {
		return nil, errors.Wrap(err, "password text field did not appear in the ui")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := kb.Type(ctx, credentials.Pass+"\n"); err != nil {
		return nil, errors.Wrap(err, "entering password failed")
	}

	// Check if the login was successful using the API and also by looking for the shelf in the UI.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
		return nil, errors.Wrapf(err, "failed waiting to log in: last state: %+v", st)
	}

	if err := ash.WaitForShelf(ctx, tLoginConn, 120*time.Second); err != nil {
		return nil, errors.Wrap(err, "shelf did not appear after logging in")
	}
	if browserType == browser.TypeLacros {
		var err error
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to test api")
		}
		// lacros.Connect() fails if DevTools connection file was already created, but Chrome does not accept connections.
		// Retry for 10 seconds.
		deadline := time.Now().Add(10 * time.Second)
		for deadline.Sub(time.Now()).Seconds() > 0 {
			l, err = lacros.Connect(ctx, tconn)
			if err == nil {
				break
			}
			testing.ContextLog(ctx, "loginPerfDoLogin: Sleeping for 10 milliseconds... ")
			err = testing.Sleep(ctx, 10*time.Millisecond)
			if err != nil {
				err = errors.Wrap(err, "failed to wait for lacros-chrome test connection")
				break
			}
		}
	}
	return l, err
}

func browserTestAPIConn(ctx context.Context, cr *chrome.Chrome, l *lacros.Lacros) (*chrome.TestConn, ash.ConnSource, error) {
	var tconn *chrome.TestConn
	var cs ash.ConnSource
	if l != nil {
		var err error
		if tconn, err = l.TestAPIConn(ctx); err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to Lacros test api")
		}
		cs = l
	} else {
		var err error
		if tconn, err = cr.TestAPIConn(ctx); err != nil {
			return nil, nil, errors.Wrap(err, "failed to connect to Ash test api")
		}
		cs = cr
	}
	return tconn, cs, nil
}

func loginPerfCreateWindows(ctx context.Context, cr *chrome.Chrome, l *lacros.Lacros, url string, n int) error {
	tconn, cs, err := browserTestAPIConn(ctx, cr, nil)
	if err != nil {
		return errors.Wrap(err, "failed to connect to browser test api")
	}
	if l != nil {
		for i := 0; i < n; i++ {
			conn, err := l.NewConn(ctx, url, browser.WithNewWindow())
			if err != nil {
				return errors.Wrapf(err, "failed to connect to the %v restore URL", url)
			}
			defer conn.Close()
		}
	} else {
		err = ash.CreateWindows(ctx, tconn, cs, url, n)
		if err != nil {
			return errors.Wrap(err, "failed to create browser windows")
		}
	}
	return nil
}

// countVisibleWindows is a proxy to ash.CountVisibleWindows(...)
func countVisibleWindows(ctx context.Context, cr *chrome.Chrome) (int, error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to connect to Ash test api")
	}
	visible, err := ash.CountVisibleWindows(ctx, tconn)
	if err != nil {
		err = errors.Wrap(err, "failed to count browser windows")
	}
	return visible, nil
}

// maxHistogramValue calculates the estimated maximum of the histogram values.
// At is an error when there are no data points.
func maxHistogramValue(h *metrics.Histogram) (float64, error) {
	if h.TotalCount() == 0 {
		return 0, errors.New("no histogram data")
	}
	var max int64 = math.MinInt64
	for _, b := range h.Buckets {
		if b.Count > 0 && max < b.Max {
			max = b.Max
		}
	}
	return float64(max), nil
}

func reportEnsureWorkVisibleHistogram(ctx context.Context, pv *perfutil.Values, hist *metrics.Histogram, unit, valueName string) error {
	value, err := maxHistogramValue(hist)
	if err != nil {
		return errors.Wrapf(err, "failed to get %s data", hist.Name)
	}
	pv.Append(perf.Metric{
		Name:      valueName,
		Unit:      unit,
		Direction: perf.SmallerIsBetter,
	}, value)
	return nil
}

// logout is a proxy to chrome.autotestPrivate.logout
func logout(ctx context.Context, cr *chrome.Chrome, l *lacros.Lacros, s *testing.State) error {
	s.Log("Sign out: started")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test api")
	}
	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return err
	}
	sw, err := sm.WatchSessionStateChanged(ctx, "stopped")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	if l != nil {
		// TODO(crbug.com/1318180): at the moment for Lacros, we're not getting SetUpWithNewChrome
		// close closure because when used it'd close all resources, including targets and wouldn't let
		// the session to proper restore later during performRegularLogin. As a short term workaround
		// we're closing Lacros resources using CloseResources fn instead, though ideally we want to
		// use SetUpWithNewChrome close closure when it's properly implemented.
		l.CloseResources(ctx)
	}

	if err := tconn.Call(ctx, nil, "chrome.autotestPrivate.logout"); err != nil {
		if errors.Is(err, rpcc.ErrConnClosing) {
			s.Log("WARNING: chrome.autotestPrivate.logout failed with: ", err)
		} else {
			return errors.Wrap(err, "failed to run chrome.autotestPrivate.logout()")
		}
	}

	select {
	case <-sw.Signals:
		s.Log("Got SessionStateChanged signal")
	case <-ctx.Done():
		s.Fatal("Didn't get SessionStateChanged signal: ", ctx.Err())
	}
	s.Log("Sign out: done")
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

	// TODO(crbug.com/1314785)
	// According to the PRD of Full Restore go/chrome-os-full-restore-dd, it uses a throttle of
	// 2.5s to save the app launching and window statue information to the backend. Therefore,
	// sleep 3 seconds here.
	testing.Sleep(ctx, 3*time.Second)
	return nil
}

func initializeLoginPerfTest(ctx context.Context, s *testing.State, browserType browser.Type, lacrosConfig *lacrosfixt.Config) (chrome.Creds, error) {
	var initArcOpt []chrome.Option
	// Only enable arc if it's supoprted.
	if arc.Supported() {
		// We enable ARC initially to fully initialize it.
		initArcOpt = []chrome.Option{chrome.ARCSupported()}
	}
	options := []chrome.Option{
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.EnableRestoreTabs(),
		chrome.SkipForceOnlineSignInForTesting(),
		chrome.EnableWebAppInstall(),
		// Disable whats-new page. See crbug.com/1271436.
		chrome.DisableFeatures("ChromeWhatsNewUI"),
	}
	if browserType == browser.TypeLacros {
		defaultOpts, err := lacrosConfig.Opts()
		if err != nil {
			return chrome.Creds{}, errors.Wrap(err, "failed to get default options")
		}
		options = append(options, defaultOpts...)
	}
	cr, err := chrome.New(
		ctx,
		append(options, initArcOpt...)...,
	)
	if err != nil {
		return chrome.Creds{}, errors.Wrap(err, "chrome login failed")
	}
	defer cr.Close(ctx)

	creds := cr.Creds()

	s.Log("Opting into Play Store")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return chrome.Creds{}, errors.Wrap(err, "failed to connect to test api")
	}
	if arc.Supported() {
		if err := optin.Perform(ctx, cr, tconn); err != nil {
			return chrome.Creds{}, errors.Wrap(err, "failed to optin to Play Store")
		}
		s.Log("Opt in finished")
	} else {
		s.Log("ARC++ is not supported. Run test without ARC")
	}
	var l *lacros.Lacros

	if browserType == browser.TypeLacros {
		var err error
		l, err = lacros.Launch(ctx, tconn)
		if err != nil {
			return chrome.Creds{}, errors.Wrap(err, "failed to launch lacros-chrome")
		}
	}
	if arc.Supported() {
		// Wait for ARC++ aps to download and initialize.
		s.Log("Initialize: let session fully initialize. Sleeping for 400 seconds... ")
		testing.Sleep(ctx, 400*time.Second)
	} else {
		s.Log("Initialiize: let session fully initialize. Sleeping for 100 seconds... ")
		testing.Sleep(ctx, 100*time.Second)
	}
	setAlwaysRestoreSettings(ctx, tconn)
	return creds, logout(ctx, cr, l, s)
}

func LoginPerf(ctx context.Context, s *testing.State) {
	param := s.Param().(testParameters)
	lacrosCfg := lacrosfixt.NewConfig(
		lacrosfixt.Selection(param.lacrosSelection),
		lacrosfixt.Mode(param.lacrosMode))

	// Log in and log out to create a user pod on the login screen.
	creds, err := initializeLoginPerfTest(ctx, s, param.bt, lacrosCfg)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}

	r := perfutil.NewRunner(nil)
	// Use 3 runs instead of 10 to reduce tests time.
	r.Runs = 3
	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	url := server.URL + "/animation.html"

	const (
		noarc        = "noarc"
		arcenabled   = "arcenabled"
		arcsupported = "arcsupported"
	)
	arcmodes := []string{noarc}
	if arc.Supported() {
		arcmodes = append(arcmodes, arcenabled, arcsupported)
	}

	currentWindows := 0
	// Run the login flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode or tablet mode.
	for _, windows := range []int{2, 8} {
		for _, arcMode := range arcmodes {
			var arcOpt []chrome.Option
			switch arcMode {
			case noarc:
			case arcenabled:
				arcOpt = []chrome.Option{chrome.ARCEnabled()}
			case arcsupported:
				arcOpt = []chrome.Option{chrome.ARCSupported()}
			default:
				s.Fatal("Unknown arcMode value=", arcMode)
			}
			s.Log("Starting test: '"+arcMode+"' for ", windows, " windows")

			if currentWindows != windows {
				s.Log("CREATE NEW WINDOWS: Sign in to create new windows")
				// Log in and log out to create a user pod on the login screen and required number of windows in session.
				err := func() error {
					// We do not need ARC to create Chrome windows.
					cr, err := loginPerfStartToLoginScreen(ctx, s, param.bt, lacrosCfg, []chrome.Option{} /*arcOpt*/, false /*useTabletMode*/)
					if err != nil {
						return err
					}
					defer cr.Close(ctx)

					var l *lacros.Lacros
					if l, err = loginPerfDoLogin(ctx, cr, creds, param.bt); err != nil {
						return err
					}
					if err := loginPerfCreateWindows(ctx, cr, l, url, windows-currentWindows); err != nil {
						return err
					}
					s.Log("Sign out: sleep for 20 seconds to let session settle")
					testing.Sleep(ctx, 20*time.Second)
					return logout(ctx, cr, l, s)
				}()
				if err != nil {
					s.Fatal("Failed to create new browser windows: ", err)
				}
				currentWindows = windows
			}

			for _, inTabletMode := range []bool{false, true} {
				var suffix string
				if inTabletMode {
					suffix = ".TabletMode"
				} else {
					suffix = ".ClamshellMode"
				}

				// cr is shared between multiple runs, because Chrome connection must to be closed only after histograms are stored.
				var cr *chrome.Chrome
				var l *lacros.Lacros

				heuristicsHistograms := []string{
					"Ash.LoginAnimation.Smoothness" + suffix,
					"Ash.LoginAnimation.Jank" + suffix,
					"Ash.LoginAnimation.Duration" + suffix,
				}
				const (
					ensureWorkVisibleHistogram       = "GPU.EnsureWorkVisibleDuration"
					ensureWorkVisibleLowResHistogram = "GPU.EnsureWorkVisibleDurationLowRes"
					allBrowserWindowsCreated         = "Ash.LoginSessionRestore.AllBrowserWindowsCreated"
					allBrowserWindowsShown           = "Ash.LoginSessionRestore.AllBrowserWindowsShown"
					allBrowserWindowsPresented       = "Ash.LoginSessionRestore.AllBrowserWindowsPresented"
					allShelfIconsLoaded              = "Ash.LoginSessionRestore.AllShelfIconsLoaded"
					shelfLoginAnimationEnd           = "Ash.LoginSessionRestore.ShelfLoginAnimationEnd"
					ashTastBootTimeLogin2            = "Ash.Tast.BootTime.Login2"
				)

				allHistograms := []string{
					ensureWorkVisibleHistogram,
					ensureWorkVisibleLowResHistogram,
					allBrowserWindowsCreated,
					allBrowserWindowsShown,
					allBrowserWindowsPresented,
					allShelfIconsLoaded,
					shelfLoginAnimationEnd,
					ashTastBootTimeLogin2,
				}
				allHistograms = append(allHistograms, heuristicsHistograms...)

				s.Logf("Starting test: '%s%s.%s.%dwindows'", s.TestName(), suffix, arcMode, currentWindows)
				r.RunMultiple(ctx, fmt.Sprintf("%dwindows%s", currentWindows, suffix),
					uiperf.Run(s, func(ctx context.Context, name string) ([]*metrics.Histogram, error) {
						var err error
						cr, err = loginPerfStartToLoginScreen(ctx, s, param.bt, lacrosCfg, arcOpt, inTabletMode)
						if err != nil {
							return nil, errors.Wrap(err, "failed to start to login screen")
						}
						tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
						if err != nil {
							return nil, errors.Wrap(err, "creating login test api connection failed")
						}
						// Shorten context a bit to allow for cleanup.
						closeCtx := ctx
						ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
						defer cancel()
						// Initialize CUJ recording.
						cujRecorder, err := cujrecorder.NewRecorderWithTestConn(ctx, tLoginConn, cr, tLoginConn, nil, cujrecorder.RecorderOptions{})
						if err != nil {
							s.Fatal("Failed to create a CUJ recorder: ", err)
						}
						defer cujRecorder.Close(closeCtx)
						// TODO(b/237400719): support lacros
						for _, metricConfig := range [][]cujrecorder.MetricConfig{cujrecorder.AshCommonMetricConfigs(), cujrecorder.BrowserCommonMetricConfigs(), cujrecorder.AnyChromeCommonMetricConfigs()} {
							if err := cujRecorder.AddCollectedMetrics(tLoginConn, browser.TypeAsh, metricConfig...); err != nil {
								s.Fatal("Failed to add recorded metrics: ", err)
							}
						}

						// The actual test function
						testFunc := func(ctx context.Context) error {
							var err error
							l, err = loginPerfDoLogin(ctx, cr, creds, param.bt)
							if err != nil {
								return errors.Wrap(err, "failed to log in")
							}
							tconn, err := cr.TestAPIConn(ctx)
							if err != nil {
								return errors.Wrap(err, "failed to connect to test api")
							}
							if err = ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
								return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
							}); err != nil {
								return errors.Wrap(err, "failed to wait")
							}
							return nil
						}

						var histograms []*metrics.Histogram
						// CUJ TPS metrics recording wrapper
						cujFunc := func(ctx context.Context) error {
							var err error
							histograms, err = metrics.RunAndWaitAll(
								ctx,
								tLoginConn,
								4*time.Minute,
								testFunc,
								allHistograms...,
							)
							if err != nil {
								return err
							}
							visible := 0
							if visible, err = countVisibleWindows(ctx, cr); err != nil {
								return err
							}
							if visible != currentWindows && visible != currentWindows+1 {
								err = errors.Errorf("unexpected number of visible windows: expected %d, found %d", currentWindows, visible)
							}
							return err
						}
						if err := cujRecorder.Run(ctx, cujFunc); err != nil {
							return nil, errors.Wrap(err, "failed to run the test scenario")
						}
						tpsValues := perf.NewValues()
						if err := cujRecorder.Record(ctx, tpsValues); err != nil {
							return nil, errors.Wrap(err, "failed to collect the data from the recorder")
						}
						r.Values().MergeWithSuffix(fmt.Sprintf("%s.%s.%dwindows", suffix, arcMode, currentWindows), tpsValues.GetValues())
						return histograms, err
					}),
					func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
						defer cr.Close(ctx)

						heuristicsHistogramsMap := make(map[string]bool, len(allHistograms))
						for _, v := range heuristicsHistograms {
							heuristicsHistogramsMap[v] = true
						}
						storeHeuristicsHistograms := perfutil.StoreAllWithHeuristics(fmt.Sprintf("%s.%dwindows", arcMode, currentWindows))

						for _, hist := range hists {
							if heuristicsHistogramsMap[hist.Name] {
								storeHeuristicsHistograms(ctx, pv, []*metrics.Histogram{hist})
							} else if hist.Name == ensureWorkVisibleHistogram || hist.Name == ensureWorkVisibleLowResHistogram || hist.Name == allBrowserWindowsCreated || hist.Name == allBrowserWindowsShown || hist.Name == allBrowserWindowsPresented || hist.Name == allShelfIconsLoaded || hist.Name == shelfLoginAnimationEnd || hist.Name == ashTastBootTimeLogin2 {
								valueName := fmt.Sprintf("%s%s.%s.%dwindows", hist.Name, suffix, arcMode, currentWindows)
								if hist.Name == ensureWorkVisibleHistogram {
									reportEnsureWorkVisibleHistogram(ctx, pv, hist, "microsecond", valueName)
								} else {
									reportEnsureWorkVisibleHistogram(ctx, pv, hist, "millisecond", valueName)
								}
							} else {
								return errors.Errorf("unknown histogram %q", hist.Name)
							}
						}
						return logout(ctx, cr, l, s)
					},
				)
			}
		}
	}
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
