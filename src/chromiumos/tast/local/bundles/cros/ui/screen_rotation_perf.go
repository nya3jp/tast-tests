// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	uiperf "chromiumos/tast/local/bundles/cros/ui/perf"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/perfutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenRotationPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures animation smoothness of screen rotation in tablet mode",
		Contacts:     []string{"chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
		Timeout: 3 * time.Minute,
	})
}

func ScreenRotationPerf(ctx context.Context, s *testing.State) {
	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(closeCtx)

	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}

	// Ensure returning back to the normal rotation at the end.
	defer display.SetDisplayRotationSync(closeCtx, tconn, dispInfo.ID, display.Rotate0)

	defer ash.SetOverviewModeAndWait(closeCtx, tconn, false)

	bt := s.Param().(browser.Type)
	url := ui.PerftestURL
	currentWindows := 0
	// Use `cr` from ash-chrome for the metrics that are recorded in ash-chrome.
	runner := perfutil.NewRunner(cr.Browser())
	// Run the screen rotation in overview mode with 2 or 8 windows.
	var br *browser.Browser
	for _, windows := range []int{2, 8} {
		// Open the first window using browserfixt to get a Browser instance, then use the browser instance to open the others.
		if currentWindows == 0 {
			var conn *browser.Conn
			var closeBrowser func(ctx context.Context) error
			conn, br, closeBrowser, err = browserfixt.SetUpWithURL(ctx, cr, bt, url)
			if err != nil {
				s.Fatal("Failed to open chrome: ", err)
			}
			defer closeBrowser(closeCtx)
			defer conn.Close()
			currentWindows++
		}
		if err := ash.CreateWindows(ctx, tconn, br, url, windows-currentWindows); err != nil {
			s.Fatal("Failed to create browser windows: ", err)
		}
		currentWindows = windows

		if err = ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
			s.Fatal("Failed to enter into the overview mode: ", err)
		}

		// Use `tconn` from ash-chrome for the metrics that are recorded in ash-chrome.
		suffix := fmt.Sprintf("%dwindows", windows)
		runner.RunMultiple(ctx, suffix, uiperf.Run(s, perfutil.RunAndWaitAll(tconn, func(ctx context.Context) error {
			for _, rotation := range []display.RotationAngle{display.Rotate90, display.Rotate180, display.Rotate270, display.Rotate0} {
				if err := display.SetDisplayRotationSync(ctx, tconn, dispInfo.ID, rotation); err != nil {
					return errors.Wrap(err, "failed to rotate display")
				}
			}
			return nil
		}, "Ash.Rotation.AnimationSmoothness")),
			perfutil.StoreAll(perf.BiggerIsBetter, "percent", suffix))
	}

	if err := runner.Values().Save(ctx, s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
