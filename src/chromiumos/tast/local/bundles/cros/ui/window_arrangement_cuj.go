// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/windowarrangementcuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowArrangementCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for window arrangements",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome", "arc", "chrome_internal"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars:         []string{"record"},
		Timeout:      20*time.Minute + cuj.CPUStablizationTimeout,
		Data:         []string{"shaka_720.webm", "pip.html", cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name: "clamshell_mode",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
				},
				Fixture:           "loggedInToCUJUser",
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "tablet_mode",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
					Tablet:      true,
				},
				Fixture:           "loggedInToCUJUser",
				ExtraSoftwareDeps: []string{"android_p"},
			},
			{
				Name: "lacros",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeLacros,
				},
				Fixture:           "loggedInToCUJUserLacros",
				ExtraSoftwareDeps: []string{"android_p", "lacros"},
			},
			{
				Name: "clamshell_mode_vm",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
				},
				Fixture:           "loggedInToCUJUser",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "tablet_mode_vm",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeAsh,
					Tablet:      true,
				},
				Fixture:           "loggedInToCUJUser",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
			{
				Name: "lacros_vm",
				Val: windowarrangementcuj.TestParam{
					BrowserType: browser.TypeLacros,
				},
				Fixture:           "loggedInToCUJUserLacros",
				ExtraSoftwareDeps: []string{"android_vm", "lacros"},
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

	// Set up the cujrecorder.Recorder: In clamshell mode, this test will measure the combinations of
	// input latency of tab dragging and of window resizing and of split view resizing, and
	// also the percent of dropped frames of video; In tablet mode, this test will measure
	// the combinations of input latency of tab dragging and of input latency of split view
	// resizing and the percent of dropped frames of video.
	var configs []cujrecorder.MetricConfig
	if !tabletMode {
		configs = append(configs, cujrecorder.NewLatencyMetricConfig("Ash.InteractiveWindowResize.TimeToPresent"))
	} else {
		configs = append(configs,
			cujrecorder.NewLatencyMetricConfig("Ash.TabDrag.PresentationTime.TabletMode"),
			cujrecorder.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.SingleWindow"),
			cujrecorder.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.WithOverview"),
			cujrecorder.NewLatencyMetricConfig("Ash.SplitViewResize.PresentationTime.TabletMode.MultiWindow"))
	}

	recorder, err := cujrecorder.NewRecorder(ctx, conns.Chrome, conns.BrowserTestConn, conns.ARC, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	if err := recorder.AddCollectedMetrics(conns.BrowserTestConn, conns.BrowserType, configs...); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	if err := recorder.AddCommonMetrics(conns.TestConn, conns.BrowserTestConn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	defer recorder.Close(closeCtx)

	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer crastestclient.Unmute(closeCtx)

	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, conns.TestConn)

	connNoPiP, err := conns.Source.NewConn(ctx, conns.PipVideoTestURL)
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connNoPiP.Close()
	// Close the browser window at the end of the test. If it is left playing a video, it
	// will cause the test server's Close() function to block for a few minutes.
	defer connNoPiP.CloseTarget(closeCtx)

	if err := webutil.WaitForQuiescence(ctx, connNoPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	connPiP, err := conns.Source.NewConn(ctx, conns.PipVideoTestURL)
	if err != nil {
		s.Fatal("Failed to load pip.html: ", err)
	}
	defer connPiP.Close()
	// Close the browser window at the end of the test. If it is left playing a video, it
	// will cause the test server's Close() function to block for a few minutes.
	defer connPiP.CloseTarget(closeCtx)

	if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
		s.Fatal("Failed to wait for pip.html to achieve quiescence: ", err)
	}

	ui := uiauto.New(conns.TestConn)

	// Only show pip window for ash-chrome.
	// TODO(crbug/1232492): Remove this after fix.
	if testParam.BrowserType == browser.TypeAsh {
		// The second tab enters the system PiP mode.
		webview := nodewith.ClassName("ContentsWebView").Role(role.WebView)
		pipButton := nodewith.Name("Enter Picture-in-Picture").Role(role.Button).Ancestor(webview)
		if err := action.Combine(
			"focus the PIP button (to ensure it is in view) and left-click on it",
			ui.FocusAndWait(pipButton),
			ui.LeftClick(pipButton),
		)(ctx); err != nil {
			s.Fatal("Failed to click the pip button: ", err)
		}
		if err := webutil.WaitForQuiescence(ctx, connPiP, timeout); err != nil {
			s.Fatal("Failed to wait for quiescence: ", err)
		}
	}

	// Lacros specific setup.
	if testParam.BrowserType == browser.TypeLacros {
		// Close blank tab created at startup after creating other tabs.
		if err := conns.CloseBlankTab(ctx); err != nil {
			s.Fatal("Failed to close blank tab: ", err)
		}
	}

	// Activate the tab on the left.
	var tabs []map[string]interface{}
	if err := conns.BrowserTestConn.Call(ctx, &tabs,
		"tast.promisify(chrome.tabs.query)",
		map[string]interface{}{},
	); err != nil {
		s.Fatal("Failed to fetch browser tabs: ", err)
	}
	if len(tabs) != 2 {
		s.Errorf("Unexpected number of browser tabs; got %d, want 2", len(tabs))
	}
	if err := conns.BrowserTestConn.Call(ctx, nil,
		"tast.promisify(chrome.tabs.update)",
		int(tabs[0]["id"].(float64)),
		map[string]interface{}{"active": true},
	); err != nil {
		s.Fatal("Failed to activate first tab: ", err)
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
			return windowarrangementcuj.RunClamShell(ctx, closeCtx, conns.TestConn, ui, pc, conns.StartARCApp, conns.StopARCApp)
		}
	} else {
		f = func(ctx context.Context) error {
			return windowarrangementcuj.RunTablet(ctx, closeCtx, conns.TestConn, ui, pc, conns.StartARCApp, conns.StopARCApp)
		}
	}

	// Run the recorder.
	if err := recorder.RunFor(ctx, f, 10*time.Minute); err != nil {
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
