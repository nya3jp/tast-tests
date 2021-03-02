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
		Desc: "Checks that an existing device user can login from the login screen",
		Contacts: []string{
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
		},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline"},
		Vars: []string{
			"ui.signinProfileTestExtensionManifestKey",
			"ui.oac_username",
			"ui.oac_password",
		},
		Timeout: 20 * time.Minute,
	})
}

// ExistingUserLogin logs in to an existing user account from the login screen.
func loginPerfStartToLoginScreen(ctx context.Context, s *testing.State, useTabletMode bool) *chrome.Chrome {
	// chrome.NoLogin() and chrome.KeepState() are needed to show the login
	// screen with a user pod (instead of the OOBE login screen).
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tLoginConn)
	defer tLoginConn.Close()

	s.Log("############ => ash.SetTabletModeEnabled(useTabletMode=", useTabletMode, ")")
	if err = ash.SetTabletModeEnabled(ctx, tLoginConn, useTabletMode); err != nil {
		s.Fatalf("Failed to set tablet mode %v: %v", useTabletMode, err)
	}

	// Wait for the login screen to be ready for password entry.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Failed waiting for the login screen to be ready for password entry: %v, last state: %+v", err, st)
	}

	return cr
}

// ExistingUserLogin logs in to an existing user account from the login screen.
func loginPerfDoLogin(ctx context.Context, s *testing.State, cr *chrome.Chrome) {
	username := s.RequiredVar("ui.oac_username")
	password := s.RequiredVar("ui.oac_password")

	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tLoginConn)
	defer tLoginConn.Close()

	// TODO(crbug/1109381): the password field isn't actually ready just yet when WaitState returns.
	// This causes it to miss some of the keyboard input, so the password will be wrong.
	// We can check in the UI for the password field to exist, which seems to be a good enough indicator that
	// the field is ready for keyboard input.
	if err := lockscreen.WaitForPasswordField(ctx, tLoginConn, username, 15*time.Second); err != nil {
		s.Fatal("Password text field did not appear in the UI: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	s.Log("Entering password to log in")
	if err := kb.Type(ctx, password+"\n"); err != nil {
		s.Fatal("Entering password failed: ", err)
	}

	// Check if the login was successful using the API and also by looking for the shelf in the UI.
	if st, err := lockscreen.WaitState(ctx, tLoginConn, func(st lockscreen.State) bool { return st.LoggedIn }, 30*time.Second); err != nil {
		s.Fatalf("Failed waiting to log in: %v, last state: %+v", err, st)
	}

	if err := ash.WaitForShelf(ctx, tLoginConn, 30*time.Second); err != nil {
		s.Fatal("Shelf did not appear after logging in: ", err)
	}
}

func loginPerfCreateWindows(ctx context.Context, s *testing.State, cr *chrome.Chrome, url string, n int) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	conns, err := ash.CreateWindows(ctx, tconn, cr, url, n)
	if err != nil {
		s.Fatal("Failed to create browser windows: ", err)
	}
	s.Log("############ loginPerfCreateWindows(): => ash.WaitWindowFinishAnimating(...)")
	if err = ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
	}); err != nil {
		s.Fatal("Failed to wait: ", err)
	}
	s.Log("############ loginPerfCreateWindows(): All windows have finished animations.")
	if err = conns.Close(); err != nil {
		s.Fatal("Failed to close connections: ", err)
	}
}

func LoginPerf(ctx context.Context, s *testing.State) {
	s.Log("############ LoginPerf(): Started")

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	s.Log("############ => setUpTestChromeLogin(ctx, s)")

	username := s.RequiredVar("ui.oac_username")
	password := s.RequiredVar("ui.oac_password")

	s.Log("############ Make sure we start with clean system.")

	// Log in and log out to create a user pod on the login screen.
	func() {
		cr, err := chrome.New(ctx, chrome.Auth(username, password, ""), chrome.GAIALogin())
		if err != nil {
			s.Fatal("Chrome login failed: ", err)
		}
		defer cr.Close(ctx)

		if err := upstart.RestartJob(ctx, "ui"); err != nil {
			s.Fatal("Failed to restart ui: ", err)
		}
	}()
	s.Log("############ Done: should be on OOBE")

	s.Log("############ r := perfutil.NewRunner(nil)")
	r := perfutil.NewRunner(nil)
	currentWindows := 0
	// Run the login flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode or tablet mode.
	s.Log("############ for i, windows := range []int{2, 8} {")
	for i, windows := range []int{2, 8} {
		s.Log("############ Iteration {i=", i, ", windows=", windows, "}: currentWindows=", currentWindows)
		if currentWindows != windows {
			// Log in and log out to create a user pod on the login screen and required number of windows in session.
			func() {
				s.Log("############ {Create enough Windows} => loginPerfStartToLoginScreen(...)")
				cr := loginPerfStartToLoginScreen(ctx, s /*useTabletMode=*/, false)
				defer cr.Close(ctx)

				s.Log("############ => loginPerfDoLogin(...)")
				loginPerfDoLogin(ctx, s, cr)
				s.Log("############ => loginPerfCreateWindows(", windows-currentWindows, ")")
				loginPerfCreateWindows(ctx, s, cr, url, windows-currentWindows)
				s.Log("############ Done: loginPerfCreateWindows(...)")

				if err := upstart.RestartJob(ctx, "ui"); err != nil {
					s.Fatal("Failed to restart ui: ", err)
				}
			}()
			currentWindows = windows
		}

		s.Log("############ for _, inTabletMode := range []bool{false, true} ...")
		for _, inTabletMode := range []bool{false, true} {
			s.Log("############ Iteration {inTabletMode=", inTabletMode, "}: for _, inTabletMode := range []bool{false, true}")

			var suffix string
			if inTabletMode {
				suffix = ".TabletMode"
			} else {
				suffix = ".ClamshellMode"
			}

			s.Log("############ => r.RunMultiple(...) ##################### suffix = '" + suffix + "'")
			var cr *chrome.Chrome

			r.RunMultiple(ctx, s, fmt.Sprintf("%dwindows%s", currentWindows, suffix), func(ctx1 context.Context) ([]*metrics.Histogram, error) {
				s.Log("############ New run.")
				s.Log("############ => loginPerfStartToLoginScreen(...)")
				cr = loginPerfStartToLoginScreen(ctx, s, inTabletMode)
				s.Log("############ => Create tconn")
				tLoginConn, err := cr.SigninProfileTestAPIConn(ctx1)
				if err != nil {
					s.Fatal("Creating login test API connection failed: ", err)
				}
				defer tLoginConn.Close()
				s.Log("############ New run. => perfutil.RunAndWaitAll(...)")
				return perfutil.RunAndWaitAll(tLoginConn, func(ctx context.Context) error {
					s.Log("############ Started: scenario func for perfutil.RunAndWaitAll.")
					s.Log("############ => loginPerfDoLogin(...)")
					loginPerfDoLogin(ctx, s, cr)
					s.Log("############ => ash.WaitWindowFinishAnimating(...)")
					tconn, err := cr.TestAPIConn(ctx1)
					if err != nil {
						s.Fatal("Failed to connect to test API: ", err)
					}
					defer tconn.Close()
					if err = ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
						return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
					}); err != nil {
						s.Fatal("Failed to wait: ", err)
					}
					s.Log("############ Done: scenario func for perfutil.RunAndWaitAll (now wait for metrics).")
					return nil
				},
					"Ash.LoginAnimation.Smoothness"+suffix,
					"Ash.LoginAnimation.Jank"+suffix,
					"Ash.LoginAnimation.Duration"+suffix)(ctx1)
			},
				func(ctx context.Context, pv *perfutil.Values, hists []*metrics.Histogram) error {
					s.Log("############ (store) Called.")
					defer cr.Close(ctx)
					perfutil.StoreAllWithHeuristics(fmt.Sprintf("%dwindows", currentWindows))(ctx, pv, hists)

					if err := upstart.RestartJob(ctx, "ui"); err != nil {
						s.Fatal("Failed to restart ui: ", err)
					}
					return nil
				})

		}
	}
	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
