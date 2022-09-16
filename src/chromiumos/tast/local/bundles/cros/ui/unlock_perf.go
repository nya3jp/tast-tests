// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/perf"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UnlockPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures animation smoothness of screen unlock",
		Contacts:     []string{"mukai@chromium.org", "oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:    "passthrough",
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedInWith100FakeAppsPassthroughCmdDecoder",
		}},
		Data: []string{"animation.html", "animation.js"},
	})
}

func UnlockPerf(ctx context.Context, s *testing.State) {
	const (
		password    = "testpass"
		lockTimeout = 30 * time.Second
		authTimeout = 30 * time.Second
	)

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed creating virtual keyboard: ", err)
	}

	defer kb.Close()

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(ctx, l)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	originalTabletMode, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the tablet mode status: ", err)
	}
	defer ash.SetTabletModeEnabled(ctx, tconn, originalTabletMode)

	// Run an http server to serve the test contents for accessing from the chrome browsers.
	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()
	url := server.URL + "/animation.html"

	r := perfutil.NewRunner(cr.Browser())
	currentWindows := 0
	// Run the unlock flow for various situations.
	// - change the number of browser windows, 2 or 8
	// - the window system status; clamshell mode or tablet mode.
	// If these window number values are changed, make sure to check lacros new tab pages are closed correctly.
	for i, windows := range []int{2, 8} {
		if err := ash.CreateWindows(ctx, tconn, cs, url, windows-currentWindows); err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}

		// This must be done after ash.CreateWindows to avoid terminating lacros-chrome.
		if i == 0 && s.Param().(browser.Type) == browser.TypeLacros {
			if err := l.Browser().CloseWithURL(ctx, chrome.NewTabURL); err != nil {
				s.Fatal("Failed to close blank tab: ", err)
			}
		}

		currentWindows = windows

		for _, inTabletMode := range []bool{false, true} {
			if err = ash.SetTabletModeEnabled(ctx, tconn, inTabletMode); err != nil {
				s.Fatalf("Failed to set tablet mode %v: %v", inTabletMode, err)
			}

			if err = ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
				return ash.WaitWindowFinishAnimating(ctx, tconn, w.ID)
			}); err != nil {
				s.Fatal("Failed to wait: ", err)
			}

			var suffix string
			if inTabletMode {
				suffix = ".TabletMode"
			} else {
				suffix = ".ClamshellMode"
			}

			r.RunMultiple(ctx, fmt.Sprintf("%dwindows%s", currentWindows, suffix), uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
				// Lock screen
				const accel = "Search+L"
				if err := kb.Accel(ctx, accel); err != nil {
					s.Fatalf("Typing %v failed: %v", accel, err)
				}
				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, lockTimeout); err != nil {
					s.Fatalf("Waiting for screen to be locked failed: %v (last status %+v)", err, st)
				}

				if err := kb.Type(ctx, password+"\n"); err != nil {
					s.Fatal("Typing correct password failed: ", err)
				}

				if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, authTimeout); err != nil {
					s.Fatalf("Waiting for screen to be unlocked failed: %v (last status %+v)", err, st)
				}

				return nil
			},
				"Ash.UnlockAnimation.Smoothness"+suffix)),
				perfutil.StoreAll(perf.BiggerIsBetter, "percent", fmt.Sprintf("%dwindows", currentWindows)))
		}
	}

	if err := r.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
