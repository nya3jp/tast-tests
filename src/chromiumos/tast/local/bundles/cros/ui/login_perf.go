// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/perfutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/upstart"
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
		// Test runs login / chrome restart 40+ times.
		Timeout: 40 * time.Minute,
		Data:    []string{"animation.html", "animation.js"},
	})
}

// loginPerfStartToLoginScreen starts Chrome to the login screen.
func loginPerfStartToLoginScreen(ctx context.Context, s *testing.State, useTabletMode bool) (cr *chrome.Chrome, hasError error) {
	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.EnableRestoreTabs(),
		chrome.SkipForceOnlineSignInForTesting(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to start Chrome.")
	}
	defer func() {
		if hasError != nil {
			cr.Close(ctx)
		}
	}()

	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Creating login test API connection failed.")
	}
	defer tLoginConn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), func() bool { return hasError != nil }, tLoginConn)

	if err = ash.SetTabletModeEnabled(ctx, tLoginConn, useTabletMode); err != nil {
		return nil, errors.Wrapf(err, "Failed to set tablet mode %v.", useTabletMode)
	}

	// Wait for the login screen to be ready for password entry.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 30*time.Second); err != nil {
		return nil, errors.Wrapf(err, "Failed waiting for the login screen to be ready for password entry: last state: %+v", st)
	}

	return cr, nil
}

// loginPerfDoLogin logs in and waits for animations finish.
func loginPerfDoLogin(ctx context.Context, s *testing.State, cr *chrome.Chrome, credentials chrome.Creds) (hasError error) {
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return errors.Wrapf(err, "Creating login test API connection failed.")
	}
	defer tLoginConn.Close()
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), func() bool { return hasError != nil }, tLoginConn)

	// TODO(crbug/1109381): the password field isn't actually ready just yet when WaitState returns.
	// This causes it to miss some of the keyboard input, so the password will be wrong.
	// We can check in the UI for the password field to exist, which seems to be a good enough indicator that
	// the field is ready for keyboard input.
	if err := lockscreen.WaitForPasswordField(ctx, tLoginConn, credentials.User, 15*time.Second); err != nil {
		return errors.Wrap(err, "Password text field did not appear in the UI.")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to get keyboard.")
	}
	defer kb.Close()

	if err := kb.Type(ctx, credentials.Pass+"\n"); err != nil {
		return errors.Wrap(err, "Entering password failed.")
	}

	// Check if the login was successful using the API and also by looking for the shelf in the UI.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "Failed waiting to log in: %v, last state: %+v", err, st)
	}

	if err := ash.WaitForShelf(ctx, tLoginConn, 30*time.Second); err != nil {
		return errors.Wrap(err, "Shelf did not appear after logging in.")
	}
	return nil
}

func loginPerfCreateWindows(ctx context.Context, cr *chrome.Chrome, url string, n int) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "Failed to connect to test API.")
	}
	defer tconn.Close()

	err = ash.CreateWindows(ctx, tconn, cr, url, n)
	if err != nil {
		return errors.Wrap(err, "Failed to create browser windows.")
	}
	return nil
}

func LoginPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	url := server.URL + "/animation.html"

	var credentials chrome.Creds

	// Log in and log out to create a user pod on the login screen.
	err := func() error {
		cr, err := chrome.New(ctx, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
		if err != nil {
			return errors.Wrap(err, "Chrome login failed.")
		}
		defer cr.Close(ctx)

		credentials = cr.Creds()

		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui: ", err)
		}
		return nil
	}()
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}

	r := perfutil.NewRunner(nil)
	currentWindows := 0
	// Run the login flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode or tablet mode.
	for _, windows := range []int{2, 8} {
		if currentWindows != windows {
			// Log in and log out to create a user pod on the login screen and required number of windows in session.
			err := func() error {
				cr, err := loginPerfStartToLoginScreen(ctx, s, false /*useTabletMode*/)
				if err != nil {
					return err
				}
				defer cr.Close(ctx)

				if err := loginPerfDoLogin(ctx, s, cr, credentials); err != nil {
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

			// cr is shared between multiple runs, because Chrome connection must to be closed after histiograms are stored.
			var cr *chrome.Chrome

			r.RunMultiple(ctx, s, fmt.Sprintf("%dwindows%s", currentWindows, suffix), func(ctx context.Context) ([]*metrics.Histogram, error) {
				var err error
				cr, err = loginPerfStartToLoginScreen(ctx, s, inTabletMode)
				tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
				if err != nil {
					return nil, errors.Wrap(err, "Creating login test API connection failed.")
				}
				defer tLoginConn.Close()
				return metrics.RunAndWaitAll(ctx, tLoginConn, time.Minute, func(ctx context.Context) error {
					err := loginPerfDoLogin(ctx, s, cr, credentials)
					if err != nil {
						return errors.Wrap(err, "Failed to log in.")
					}
					tconn, err := cr.TestAPIConn(ctx)
					if err != nil {
						return errors.Wrap(err, "Failed to connect to test API.")
					}
					defer tconn.Close()
					if err = ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
						return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
					}); err != nil {
						return errors.Wrap(err, "Failed to wait.")
					}
					return nil
				},
					"Ash.LoginAnimation.Smoothness"+suffix,
					"Ash.LoginAnimation.Jank"+suffix,
					"Ash.LoginAnimation.Duration"+suffix)
			},
				func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
					defer cr.Close(ctx)
					perfutil.StoreAllWithHeuristics(fmt.Sprintf("%dwindows", currentWindows))(ctx, pv, hists)
					return nil
				})

		}
	}
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
