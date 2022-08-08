// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/arcpipvideotest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type arcPIPEnergyAndPowerTestParams struct {
	bigPIP      bool
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIPEnergyAndPower,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures energy and power usage of ARC++ PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"bear-320x240.h264.mp4"},
		Timeout:      6 * time.Minute,
		Params: []testing.Param{{
			Name:              "small",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: false, browserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
		}, {
			Name:              "big",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: true, browserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
		}, {
			Name:              "small_lacros",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: false, browserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}, {
			Name:              "big_lacros",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: true, browserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"android_p", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}, {
			Name:              "small_vm",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: false, browserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
		}, {
			Name:              "big_vm",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: true, browserType: browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
		}, {
			Name:              "small_lacros_vm",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: false, browserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}, {
			Name:              "big_lacros_vm",
			Val:               arcPIPEnergyAndPowerTestParams{bigPIP: true, browserType: browser.TypeLacros},
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			Fixture:           "lacrosWithArcBooted",
		}},
	})
}

func PIPEnergyAndPower(ctx context.Context, s *testing.State) {
	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	params := s.Param().(arcPIPEnergyAndPowerTestParams)
	cr := s.FixtValue().(*arc.PreData).Chrome

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, params.browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	var browserWindowType ash.WindowType
	switch params.browserType {
	case browser.TypeAsh:
		browserWindowType = ash.WindowTypeBrowser
	case browser.TypeLacros:
		browserWindowType = ash.WindowTypeLacros
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard event writer: ", err)
	}
	defer kw.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if _, err := cpu.WaitUntilCoolDown(ctx, cpu.DefaultCoolDownConfig(cpu.CoolDownPreserveUI)); err != nil {
		s.Fatal("Failed to wait for CPU to cool down: ", err)
	}

	cleanUp, err := arcpipvideotest.EstablishARCPIPVideo(ctx, tconn, s.FixtValue().(*arc.PreData).ARC, s.DataFileSystem(), params.bigPIP)
	if err != nil {
		s.Fatal("Failed to establish ARC PIP video: ", err)
	}
	defer cleanUp(cleanupCtx)

	conn, err := br.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	// Tab away from the search box of chrome://settings, so that
	// there will be no blinking cursor.
	if err := kw.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to send Tab: ", err)
	}

	brw, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.WindowType == browserWindowType })
	if err != nil {
		s.Fatal("Failed to get browser window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, brw.ID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize browser window: ", err)
	}

	// triedToStopTracing means that cr.StopTracing(cleanupCtx)
	// was already done, with or without success (if it failed
	// then we have no reason to try again with the same timeout).
	triedToStopTracing := false
	defer func() {
		if triedToStopTracing {
			return
		}
		if _, err := cr.StopTracing(cleanupCtx); err != nil {
			s.Error("Failed to stop tracing viz.triangles in cleanup phase: ", err)
		}
	}()
	// At this time, systrace causes kernel crash on dedede devices. Because of
	// that and data points from systrace isn't actually helpful to most of
	// UI tests, disable systraces for the time being.
	// TODO(https://crbug.com/1162385, b/177636800): enable it.
	if err := cr.StartTracing(ctx, []string{"disabled-by-default-viz.triangles"}, browser.DisableSystrace()); err != nil {
		s.Fatal("Failed to start tracing viz.triangles: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	const timelineDuration = time.Minute
	if err := testing.Sleep(ctx, timelineDuration); err != nil {
		s.Fatalf("Failed to wait %v: %v", timelineDuration, err)
	}

	pv, err := timeline.StopRecording(ctx)
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}

	// As we still have to save results to files, we are not yet
	// focusing on cleanup, but we can safely pass cleanupCtx
	// (borrowing from the time reserved for cleanup) because
	// StopTracing was deferred to cleanup and we are now getting
	// it done ahead of time (see comment on triedToStopTracing).
	triedToStopTracing = true
	tr, err := cr.StopTracing(cleanupCtx)
	if err != nil {
		s.Fatal("Failed to stop tracing viz.triangles: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}

	if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(s.OutDir(), "trace.data.gz")); err != nil {
		s.Error("Failed to save trace data: ", err)
	}
}
