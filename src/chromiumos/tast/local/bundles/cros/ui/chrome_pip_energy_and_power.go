// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type chromePIPEnergyAndPowerTestParams struct {
	bigPIP      bool
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromePIPEnergyAndPower,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures energy and power usage of Chrome PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perf@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "proprietary_codecs"},
		Data:         []string{"bear-320x240.h264.mp4", "pip_video.html"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name:    "small",
			Fixture: "chromeLoggedIn",
			Val:     chromePIPEnergyAndPowerTestParams{bigPIP: false, browserType: browser.TypeAsh},
		}, {
			Name:    "big",
			Fixture: "chromeLoggedIn",
			Val:     chromePIPEnergyAndPowerTestParams{bigPIP: true, browserType: browser.TypeAsh},
		}, {
			Name:              "small_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
			Val:               chromePIPEnergyAndPowerTestParams{bigPIP: false, browserType: browser.TypeLacros},
		}, {
			Name:              "big_lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
			Val:               chromePIPEnergyAndPowerTestParams{bigPIP: true, browserType: browser.TypeLacros},
		}},
	})
}

func ChromePIPEnergyAndPower(ctx context.Context, s *testing.State) {
	params := s.Param().(chromePIPEnergyAndPowerTestParams)
	var pipClassName, settingsTitle string
	switch params.browserType {
	case browser.TypeAsh:
		pipClassName = "PictureInPictureWindow"
		settingsTitle = "Chrome - Settings"
	case browser.TypeLacros:
		pipClassName = "Widget"
		settingsTitle = "Settings - Google Chrome"
	}

	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr, l, cs, err := lacros.Setup(ctx, s.FixtValue(), params.browserType)
	if err != nil {
		s.Fatal("Failed to initialize test: ", err)
	}
	defer lacros.CloseLacros(cleanupCtx, l)

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

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
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

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()

	conn, err := cs.NewConn(ctx, srv.URL+"/pip_video.html")
	if err != nil {
		s.Fatal("Failed to load pip_video.html: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for pip_video.html to achieve quiescence: ", err)
	}

	ac := uiauto.New(tconn)

	pipButton := nodewith.Name("PIP").Role(role.Button)
	pipWindow := nodewith.Name("Picture in picture").ClassName(pipClassName).Onscreen().First()

	if err := action.Combine(
		"show PIP window",
		ac.LeftClick(pipButton),
		ac.WithTimeout(10*time.Second).WaitUntilExists(pipWindow),
	)(ctx); err != nil {
		s.Fatal("Failed to show the PIP window: ", err)
	}

	pipWindowBounds, err := ac.Location(ctx, pipWindow)
	if err != nil {
		s.Fatal("Failed to get the PIP window location (before resize): ", err)
	}

	workAreaTopLeft := info.WorkArea.TopLeft()
	var resizeEnd coords.Point
	if params.bigPIP {
		resizeEnd = workAreaTopLeft
	} else {
		resizeEnd = info.WorkArea.BottomRight().Sub(coords.NewPoint(1, 1))
	}

	if err := action.Combine(
		"resize the PIP window",
		mouse.Move(tconn, pipWindowBounds.TopLeft(), 0),
		mouse.Press(tconn, mouse.LeftButton),
		mouse.Move(tconn, resizeEnd, time.Second),
		mouse.Release(tconn, mouse.LeftButton),
	)(ctx); err != nil {
		// Ensure releasing the mouse button.
		if err := mouse.Release(tconn, mouse.LeftButton)(cleanupCtx); err != nil {
			s.Error("Failed to release the mouse button: ", err)
		}
		s.Fatal("Failed to resize the PIP window: ", err)
	}

	pipWindowBounds, err = ac.Location(ctx, pipWindow)
	if err != nil {
		s.Fatal("Failed to get the PIP window location (after resize): ", err)
	}

	if params.bigPIP {
		// For code maintainability, just check a relatively permissive expectation for
		// the maximum size of the PIP window: it should be either strictly wider than 2/5
		// of the work area width, or strictly taller than 2/5 of the work area height.
		if 5*pipWindowBounds.Width <= 2*info.WorkArea.Width && 5*pipWindowBounds.Height <= 2*info.WorkArea.Height {
			s.Fatalf("Expected a bigger PIP window. Got a %v PIP window in a %v work area", pipWindowBounds.Size(), info.WorkArea.Size())
		}
	} else {
		// The minimum size of a Chrome PIP window is 260x146. The aspect ratio of the
		// video is 4x3, and so the minimum width 260 corresponds to a height of 195.
		if pipWindowBounds.Width != 260 || pipWindowBounds.Height != 195 {
			s.Fatalf("PIP window is %v. It should be (260 x 195)", pipWindowBounds.Size())
		}
	}

	// Ensure that the PIP window will show no controls or resize shadows.
	if err := mouse.Move(tconn, workAreaTopLeft.Add(coords.NewPoint(20, 20)), time.Second)(ctx); err != nil {
		s.Fatal("Failed to move mouse: ", err)
	}

	extraConn, err := cs.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer extraConn.Close()

	if err := webutil.WaitForQuiescence(ctx, extraConn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	// Tab away from the search box of chrome://settings, so that
	// there will be no blinking cursor.
	if err := kw.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to send Tab: ", err)
	}

	br, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.Title == settingsTitle })
	if err != nil {
		s.Fatal("Failed to get main browser window: ", err)
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, br.ID, ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to maximize main browser window: ", err)
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
