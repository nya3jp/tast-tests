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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/lacros"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowArrangementCUJ,
		Desc:         "Measures the performance of critical user journey for window arrangements",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars:         []string{"record"},
		Timeout:      10 * time.Minute,
		Data:         []string{"bear-320x240.vp8.webm", "pip.html"},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val: windowarrangementcuj.TestParam{
					ChromeType: lacros.ChromeTypeChromeOS,
				},
				Fixture: "chromeLoggedIn",
			},
			{
				Name: "tablet_mode",
				Val: windowarrangementcuj.TestParam{
					ChromeType: lacros.ChromeTypeChromeOS,
					Tablet:     true,
				},
			},
			{
				Name: "lacros",
				Val: windowarrangementcuj.TestParam{
					ChromeType: lacros.ChromeTypeLacros,
				},
				Fixture:           "lacrosStartedByDataUI",
				ExtraData:         []string{launcher.DataArtifact},
				ExtraSoftwareDeps: []string{"lacros"},
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

	cs, tconn, chromeCleanUp, closeAboutBlank, err := windowarrangementcuj.SetupChrome(ctx, s)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer chromeCleanUp(closeCtx)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure clamshell/tablet mode: ", err)
	}
	defer cleanup(closeCtx)

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("record"); ok {
		screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
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
		screenRecorder.Start(ctx, tconn)
	}

	// Set up the cuj.Recorder: In clamshell mode, this test will measure the combinations of
	// input latency of tab dragging and of window resizing and of split view resizing, and
	// also the percent of dropped frames of video; In tablet mode, this test will measure
	// the combinations of input latency of tab dragging and of input latency of split view
	// resizing and the percent of dropped frames of video.
	var configs []cuj.MetricConfig
	if !tabletMode {
		configs = []cuj.MetricConfig{
			cuj.NewLatencyMetricConfig("Ash.TabDrag.PresentationTime.ClamshellMode"),
			cuj.NewLatencyMetricConfig("Ash.InteractiveWindowResize.TimeToPresent"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.ClamshellMode.SingleWindow"),
			cuj.NewCustomMetricConfig(
				"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
				"percent", perf.SmallerIsBetter, []int64{50, 80}),
		}
	} else {
		configs = []cuj.MetricConfig{
			cuj.NewLatencyMetricConfig("Ash.TabDrag.PresentationTime.TabletMode"),
			cuj.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.MultiWindow"),
			cuj.NewCustomMetricConfig(
				"Graphics.Smoothness.PercentDroppedFrames.CompositorThread.Video",
				"percent", perf.SmallerIsBetter, []int64{50, 80}),
		}
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer crastestclient.Unmute(closeCtx)

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	connPiP, err := cs.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connPiP.Close()
	if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	connNoPiP, err := cs.NewConn(ctx, srv.URL+"/pip.html")
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connNoPiP.Close()
	if err := webutil.WaitForQuiescence(ctx, connNoPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	ui := uiauto.New(tconn)

	// Only show pip window for ash-chrome.
	// TODO(crbug/1232492): Remove this after fix.
	if testParam.ChromeType == lacros.ChromeTypeChromeOS {
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
	if testParam.ChromeType == lacros.ChromeTypeLacros {
		// Close about:blank created at startup after creating other tabs.
		if err := closeAboutBlank(ctx); err != nil {
			s.Fatal("Failed to close about:blank: ", err)
		}
	}

	var pc pointer.Context
	if !tabletMode {
		pc = pointer.NewMouse(tconn)
	} else {
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller: ", err)
		}
	}
	defer pc.Close()

	var f func(ctx context.Context) error
	if !tabletMode {
		f = func(ctx context.Context) error {
			return windowarrangementcuj.RunClamShell(ctx, tconn, ui, pc)
		}
	} else {
		f = func(ctx context.Context) error {
			return windowarrangementcuj.RunTablet(ctx, tconn, ui, pc)
		}
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
