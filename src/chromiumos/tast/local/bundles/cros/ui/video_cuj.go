// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCUJ,
		Desc:         "Measures the smoothess of switch between full screen video and a tab/app",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "clamshell",
			Val:  false,
		}, {
			Name:              "tablet",
			Val:               true,
			ExtraSoftwareDeps: []string{"tablet_mode"},
		}},
	})
}

func VideoCUJ(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if err := audio.Mute(ctx); err != nil {
		s.Fatal("Failed to mute audio: ", err)
	}
	defer audio.Unmute(ctx)

	tabletMode := s.Param().(bool)
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
	}
	defer cleanup(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	tsw, err := input.Touchscreen(ctx)
	if err != nil {
		s.Fatal("Failed to open touchscreen device: ", err)
	}
	defer tsw.Close()
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the display orientation: ", err)
	}
	if err = tsw.SetRotation(-orientation.Angle); err != nil {
		s.Fatal("Failed to set rotation: ", err)
	}
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the internal display info: ", err)
	}
	tcc := tsw.NewTouchCoordConverter(info.Bounds.Size())

	stw, err := tsw.NewSingleTouchWriter()
	if err != nil {
		s.Fatal("Failed to create single touch writer: ", err)
	}
	defer stw.Close()

	webConn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open web: ", err)
	}
	defer webConn.Close()

	ytConn, err := cr.NewConn(ctx,
		"https://www.youtube.com/watch?v=EEIk7gwjgIM",
		cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open youtube: ", err)
	}
	defer func() {
		// Leaving a video running could be annoying for developers.
		ytConn.CloseTarget(ctx)
		ytConn.Close()
	}()

	// Wait for <video> tag to show up.
	if err := ytConn.WaitForExpr(ctx,
		`!!document.querySelector("video")`); err != nil {
		s.Fatal("Failed to wait for video element: ", err)
	}

	s.Log("Making video fullscreen")
	if err := ytConn.Eval(ctx,
		`document.querySelector("video").requestFullscreen()`, nil); err != nil {
		s.Fatal("Failed to make video fullscreen: ", err)
	}

	var ytWinID int
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ytWin, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.State == ash.WindowStateFullscreen
		})
		if ytWin != nil {
			ytWinID = ytWin.ID
		}
		return err
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to wait for fullscreen: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.startSmoothnessTracking)()`, nil); err != nil {
		s.Fatal("Failed to start display smoothness tracking: ", err)
	}

	s.Log("Switch away from fullscreen video")
	if tabletMode {
		if err := ytConn.Eval(ctx, `document.exitFullscreen();`, nil); err != nil {
			s.Fatal("Failed to make video fullscreen: ", err)
		}

		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			s.Fatal("Failed to DragToShowOverview: ", err)
		}

		w, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.ID != ytWinID
		})
		if err != nil {
			s.Fatal("Failed to find the other window: ", err)
		}

		tapX, tapY := tcc.ConvertLocation(w.OverviewInfo.Bounds.CenterPoint())
		if err := stw.Move(tapX, tapY); err != nil {
			s.Fatal("Failed to touch down the other window's overview item: ", err)
		}
		if err := stw.End(); err != nil {
			s.Fatal("Failed to touch up the other window's overview item: ", err)
		}
	} else {
		if err := kb.Accel(ctx, "Alt+Tab"); err != nil {
			s.Fatal("Failed to type tab: ", err)
		}
	}

	// Verify the youtube window is no longer active.
	if ytWin, err := ash.GetWindow(ctx, tconn, ytWinID); err != nil {
		s.Fatal("Failed to get youtube window: ", err)
	} else if ytWin.IsActive {
		s.Fatal("Failed to switch away from youtube window")
	}

	s.Log("Switch back to fullscreen video")
	if tabletMode {
		if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
			s.Fatal("Failed to DragToShowOverview: ", err)
		}

		if ytWin, err := ash.GetWindow(ctx, tconn, ytWinID); err != nil {
			s.Fatal("Failed to get youtube window: ", err)
		} else {
			tapX, tapY := tcc.ConvertLocation(ytWin.OverviewInfo.Bounds.CenterPoint())
			if err := stw.Move(tapX, tapY); err != nil {
				s.Fatal("Failed to touch down the other window's overview item: ", err)
			}
			if err := stw.End(); err != nil {
				s.Fatal("Failed to touch up the other window's overview item: ", err)
			}
		}

		if err := ytConn.Eval(ctx,
			`document.querySelector("video").requestFullscreen()`, nil); err != nil {
			s.Fatal("Failed to make video fullscreen: ", err)
		}
	} else {
		if err := kb.Accel(ctx, "Alt+Tab"); err != nil {
			s.Fatal("Failed to type tab: ", err)
		}
	}

	// Verify the youtube window is active and fullscreen again.
	if ytWin, err := ash.GetWindow(ctx, tconn, ytWinID); err != nil {
		s.Fatal("Failed to get youtube window: ", err)
	} else if !ytWin.IsActive || ytWin.State != ash.WindowStateFullscreen {
		s.Fatal("Failed to switch back to fullscreen youtube window")
	}

	// Calculate display smoothness.
	s.Log("Getting display smoothness")
	var ds float64
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.stopSmoothnessTracking)()`, &ds); err != nil {
		s.Fatal("Failed to stop display smoothness tracking: ", err)
	}
	s.Log("Display smoothness: ", ds)

	// Get video smoothness.
	s.Log("Getting video smoothness")
	var vs float64
	if err := ytConn.Eval(ctx,
		`(function() {
			var q = document.querySelector("video").getVideoPlaybackQuality();
			var d = q.droppedVideoFrames * 100 / q.totalVideoFrames;
			return Math.round(100 - d);
		})()`, &vs); err != nil {
		s.Fatal("Failed to get video smoothness: ", err)
	}
	s.Log("Video smoothness: ", vs)

	metricSuffix := "clamshell"
	if tabletMode {
		metricSuffix = "tablet"
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "VideoCUJ.DisplaySmoothness." + metricSuffix,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, ds)
	pv.Set(perf.Metric{
		Name:      "VideoCUJ.VideoSmoothness." + metricSuffix,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, vs)

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
