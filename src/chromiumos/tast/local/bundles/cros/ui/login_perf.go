// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LoginPerf,
		Desc: "Measures animation smoothness of screen unlock",
		Contacts: []string{
			"alemate@google.com",
			"oshima@google.com",
			"chromeos-wmp@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Vars: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.gaiaPoolDefault",
		},
		// Test runs login / chrome restart 60+ times.
		Timeout: 60 * time.Minute,
		Data:    []string{"animation.html", "animation.js"},
	})
}

// loginPerfStartToLoginScreen starts Chrome to the login screen.
func loginPerfStartToLoginScreen(ctx context.Context, s *testing.State, arcOpt []chrome.Option, useTabletMode bool) (cr *chrome.Chrome, retErr error) {
	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
	options := []chrome.Option{
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.EnableRestoreTabs(),
		chrome.SkipForceOnlineSignInForTesting(),
		chrome.EnableWebAppInstall(),
	}
	options = append(options, arcOpt...)
	cr, err := chrome.New(
		ctx,
		options...,
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
	defer tLoginConn.Close()
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
func loginPerfDoLogin(ctx context.Context, cr *chrome.Chrome, credentials chrome.Creds) (retErr error) {
	outdir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("no output directory exists")
	}
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "creating login test API connection failed")
	}
	defer tLoginConn.Close()
	defer faillog.DumpUITreeOnError(ctx, outdir, func() bool { return retErr != nil }, tLoginConn)

	// TODO(crbug/1109381): the password field isn't actually ready just yet when WaitState returns.
	// This causes it to miss some of the keyboard input, so the password will be wrong.
	// We can check in the UI for the password field to exist, which seems to be a good enough indicator that
	// the field is ready for keyboard input.
	if err := lockscreen.WaitForPasswordField(ctx, tLoginConn, credentials.User, 15*time.Second); err != nil {
		return errors.Wrap(err, "password text field did not appear in the ui")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	if err := kb.Type(ctx, credentials.Pass+"\n"); err != nil {
		return errors.Wrap(err, "entering password failed")
	}

	// Check if the login was successful using the API and also by looking for the shelf in the UI.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "failed waiting to log in: %v, last state: %+v", err, st)
	}

	if err := ash.WaitForShelf(ctx, tLoginConn, 30*time.Second); err != nil {
		return errors.Wrap(err, "shelf did not appear after logging in")
	}
	return nil
}

func loginPerfCreateWindows(ctx context.Context, cr *chrome.Chrome, url string, n int) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to test api")
	}
	defer tconn.Close()

	err = ash.CreateWindows(ctx, tconn, cr, url, n)
	if err != nil {
		return errors.Wrap(err, "failed to create browser windows")
	}
	return nil
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

func LoginPerf(ctx context.Context, s *testing.State) {
	// Log in and log out to create a user pod on the login screen.
	creds, err := func() (chrome.Creds, error) {
		cr, err := chrome.New(ctx,
			chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
			chrome.EnableRestoreTabs(),
			chrome.SkipForceOnlineSignInForTesting(),
			chrome.EnableWebAppInstall(),
			// We anable ARC initially to fully initialize it.
			chrome.ARCSupported(),
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
		defer tconn.Close()
		//if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		if err := optin.Perform(ctx, cr, tconn); err != nil {
			return chrome.Creds{}, errors.Wrap(err, "failed to optin to Play Store")
		}
		s.Log("Opt in finished")
		// Wait for ARC++ aps to download and initialize.
		s.Log("Initialiize: let session fully initialize. Sleeping for 400 seconds.. ")
		testing.Sleep(ctx, 400*time.Second)

		return creds, nil
	}()
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	r := perfutil.NewRunner(nil)
	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	url := server.URL + "/animation.html"

	const (
		noarc        = "noarc"
		arcenabled   = "arcenabled"
		arcsupported = "arcsupported"
	)
	currentWindows := 0
	// Run the login flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode or tablet mode.
	for _, windows := range []int{2, 8} {
		for _, arcMode := range []string{noarc, arcenabled, arcsupported} {
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
				// Log in and log out to create a user pod on the login screen and required number of windows in session.
				err := func() error {
					// We do not need ARC to create Chrome windows.
					cr, err := loginPerfStartToLoginScreen(ctx, s, []chrome.Option{} /*arcOpt*/, false /*useTabletMode*/)
					if err != nil {
						return err
					}
					defer cr.Close(ctx)

					if err := loginPerfDoLogin(ctx, cr, creds); err != nil {
						return err
					}
					if err := loginPerfCreateWindows(ctx, cr, url, windows-currentWindows); err != nil {
						return err
					}
					return nil
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

				// cr is shared between multiple runs, because Chrome connection must to be closed only after histiograms are stored.
				var cr *chrome.Chrome

				heuristicsHistograms := []string{
					"Ash.LoginAnimation.Smoothness" + suffix,
					"Ash.LoginAnimation.Jank" + suffix,
					"Ash.LoginAnimation.Duration" + suffix,
				}
				const (
					ensureWorkVisibleHistogram = "GPU.EnsureWorkVisibleDuration"
				)

				allHistograms := []string{ensureWorkVisibleHistogram}
				allHistograms = append(allHistograms, heuristicsHistograms...)

				r.RunMultiple(ctx, s,
					fmt.Sprintf("%dwindows%s", currentWindows, suffix),
					func(ctx context.Context) ([]*metrics.Histogram, error) {
						var err error
						cr, err = loginPerfStartToLoginScreen(ctx, s, arcOpt, inTabletMode)
						if err != nil {
							return nil, errors.Wrap(err, "failed to start to login screen")
						}
						tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
						if err != nil {
							return nil, errors.Wrap(err, "creating login test api connection failed")
						}
						defer tLoginConn.Close()
						testFunc := func(ctx context.Context) error {
							err := loginPerfDoLogin(ctx, cr, creds)
							if err != nil {
								return errors.Wrap(err, "failed to log in")
							}
							tconn, err := cr.TestAPIConn(ctx)
							if err != nil {
								return errors.Wrap(err, "failed to connect to test api")
							}
							defer tconn.Close()
							if err = ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
								return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
							}); err != nil {
								return errors.Wrap(err, "failed to wait")
							}
							return nil
						}
						return metrics.RunAndWaitAll(
							ctx,
							tLoginConn,
							time.Minute,
							testFunc,
							allHistograms...,
						)
					},
					func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
						defer cr.Close(ctx)

						heuristicsHistogramsMap := make(map[string]bool, len(allHistograms))
						for _, v := range heuristicsHistograms {
							heuristicsHistogramsMap[v] = true
						}
						storeHeuristicsHistograms := perfutil.StoreAllWithHeuristics(fmt.Sprintf("%dwindows", currentWindows))

						for _, hist := range hists {
							if heuristicsHistogramsMap[hist.Name] {
								storeHeuristicsHistograms(ctx, pv, []*metrics.Histogram{hist})
								continue
							}
							if hist.Name == ensureWorkVisibleHistogram {
								value, err := maxHistogramValue(hist)
								if err != nil {
									continue
								}
								name := hist.Name + "." + suffix
								testing.ContextLog(ctx, name, " = ", value)
								pv.Append(perf.Metric{
									Name:      name,
									Unit:      "microsecond",
									Direction: perf.SmallerIsBetter,
								}, value)
								continue
							}
							return errors.Errorf("unknown histogram %q", hist.Name)
						}
						return nil
					},
				)

			}
		}
	}
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
