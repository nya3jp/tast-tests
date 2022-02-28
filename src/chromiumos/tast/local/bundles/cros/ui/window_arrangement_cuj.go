// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/windowarrangementcuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowArrangementCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for window arrangements",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars:         []string{"record"},
		Timeout:      10*time.Minute + cuj.CPUStablizationTimeout,
		Data:         []string{"bear-320x240.vp8.webm", "pip.html"},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
				},
				Fixture:           "arcBootedInClamshellMode",
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_mode",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
					Tablet:      true,
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_mode_trace",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
					Tablet:      true,
					Tracing:     true,
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_mode_validation",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
					Tablet:      true,
					Validation:  true,
				},
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "lacros",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeLacros,
				},
				Fixture:           "lacrosWithArcBooted",
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
			},
			{
				Name: "vm",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
				},
				Fixture:           "arcBootedInClamshellMode",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func WindowArrangementCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout  = 10 * time.Second
		duration = 2 * time.Second
	)

	// Ensure display on to record ui performance correctly.
	if err := power.TurnOnDisplay(ctx); err != nil {
		s.Fatal("Failed to turn on display: ", err)
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	testParam := s.Param().(windowarrangementcuj.TestParam)
	tabletMode := testParam.Tablet

	conns, err := windowarrangementcuj.SetupChrome(ctx, closeCtx, s)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer conns.Cleanup(closeCtx)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, conns.TestConn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(closeCtx)

	// Wait for CPU to stabilize before test.
	if err := cpu.WaitUntilStabilized(ctx, cuj.CPUCoolDownConfig()); err != nil {
		// Log the cpu stabilizing wait failure instead of make it fatal.
		// TODO(b/213238698): Include the error as part of test data.
		s.Log("Failed to wait for CPU to become idle: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, conns.TestConn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, conns.TestConn)
		if err != nil {
			s.Fatal("Failed to create ScreenRecorder: ", err)
		}
		defer func() {
			screenRecorder.Stop(ctx)
			dir, ok := testing.ContextOutDir(ctx)
			if ok && dir != "" {
				if _, err := os.Stat(dir); err == nil {
					testing.ContextLogf(ctx, "Saving screen record to %s", dir)
					if err := screenRecorder.SaveInBytes(ctx, filepath.Join(dir, "screen_record.webm")); err != nil {
						s.Fatal("Failed to save screen record in bytes: ", err)
					}
				}
			}
			screenRecorder.Release(ctx)
		}()
		screenRecorder.Start(ctx, conns.TestConn)
	}

	// Set up the cuj.Recorder: In clamshell mode, this test will measure the combinations of
	// input latency of tab dragging and of window resizing and of split view resizing, and
	// also the percent of dropped frames of video; In tablet mode, this test will measure
	// the combinations of input latency of tab dragging and of input latency of split view
	// resizing and the percent of dropped frames of video.
	configs := []cuj.MetricConfig{
		// Ash metrics config, always collected from ash-chrome.
		cuj.NewCustomMetricConfig(
			"Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent",
			perf.SmallerIsBetter, []int64{50, 80}),
		cuj.NewCustomMetricConfig(
			"Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks",
			perf.SmallerIsBetter, []int64{0, 3}),
	}
	if !tabletMode {
		configs = []cuj.MetricConfig{
			cuj.NewLatencyMetricConfig("Ash.TabDrag.PresentationTime.ClamshellMode"),
			cuj.NewLatencyMetricConfig("Ash.InteractiveWindowResize.TimeToPresent"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.WithOverview"),
			cuj.NewCustomMetricConfigWithTestConn(
				"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
				"percent", perf.SmallerIsBetter, []int64{50, 80}, conns.BrowserTestConn),
		}
	} else {
		configs = []cuj.MetricConfig{
			cuj.NewLatencyMetricConfig("Ash.TabDrag.PresentationTime.TabletMode"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.SingleWindow"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.MultiWindow"),
			cuj.NewCustomMetricConfigWithTestConn(
				"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
				"percent", perf.SmallerIsBetter, []int64{50, 80}, conns.BrowserTestConn),
		}
	}

	recorder, err := cuj.NewRecorder(ctx, conns.Chrome, nil, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	if testParam.Tracing {
		recorder.EnableTracing(s.OutDir())
	}
	defer recorder.Close(closeCtx)

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer crastestclient.Unmute(closeCtx)

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, conns.TestConn)

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	connPiP, err := conns.Source.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connPiP.Close()
	if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	connNoPiP, err := conns.Source.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connNoPiP.Close()
	if err := webutil.WaitForQuiescence(ctx, connNoPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	ui := uiauto.New(conns.TestConn)

	// Only show pip window for ash-chrome.
	// TODO(crbug/1232492): Remove this after fix.
	if testParam.BrowserType == browser.TypeAsh {
		// The second tab enters the system PiP mode.
		webview := nodewith.ClassName("ContentsWebView").Role(role.WebView)
		pipButton := nodewith.Name("Enter Picture-in-Picture").Role(role.Button).Ancestor(webview)
		if err := ui.LeftClick(pipButton)(ctx); err != nil {
			s.Fatal("Failed to click the pip button: ", err)
		}
		if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
			s.Fatal("Failed to wait for quiescence: ", err)
		}
	}

	// Lacros specific setup.
	if testParam.BrowserType == browser.TypeLacros {
		// Close about:blank created at startup after creating other tabs.
		if err := conns.CloseAboutBlank(ctx); err != nil {
			s.Fatal("Failed to close about:blank: ", err)
		}
	}

	var pc pointer.Context
	if !tabletMode {
		pc = pointer.NewMouse(conns.TestConn)
	} else {
		pc, err = pointer.NewTouch(ctx, conns.TestConn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	}
	defer pc.Close()

	var f func(ctx context.Context) error
	if !tabletMode {
		f = func(ctx context.Context) error {
			return windowarrangementcuj.RunClamShell(ctx, conns.TestConn, ui, pc)
		}
	} else {
		f = func(ctx context.Context) error {
			return windowarrangementcuj.RunTablet(ctx, conns.TestConn, ui, pc)
		}
	}

	if testParam.Validation {
		validationHelper := cuj.NewTPSValidationHelper(closeCtx)
		if err := validationHelper.Stress(); err != nil {
			s.Fatal("Failed to stress: ", err)
		}
		defer func() {
			if err := validationHelper.Release(); err != nil {
				s.Fatal("Failed to release validationHelper: ", err)
			}
		}()
	}

	// Run the recorder.
	if err := recorder.Run(ctx, f); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	// Check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	// Store perf metrics.
	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save the perf data: ", err)
	}
}
