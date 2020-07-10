// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/pointer"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCUJ,
		Desc:         "Measures the smoothess of switch between full screen video and a tab/app",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      4 * time.Minute,
		Pre:          cuj.LoggedInToCUJUser(),
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"ui.VideoCUJ.ytExperiments",
		},
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
	cr := s.PreValue().(cuj.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
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

	var pc pointer.Controller
	var tsw *input.TouchscreenEventWriter
	var stw *input.SingleTouchEventWriter
	if tabletMode {
		tc, err := pointer.NewTouchController(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller")
		}
		pc = tc

		tsw = tc.Touchscreen()
		stw = tc.EventWriter()
	} else {
		pc = pointer.NewMouseController(tconn)
	}
	defer pc.Close()

	webConn, err := cr.NewConn(ctx, ui.PerftestURL)
	if err != nil {
		s.Fatal("Failed to open web: ", err)
	}
	defer webConn.Close()

	var webWinID int
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get all window: ", err)
	} else if len(all) != 1 {
		s.Fatalf("Expect 1 window, got %d", len(all))
	} else {
		webWinID = all[0].ID
	}

	s.Log("Open youtube Web")
	ytConn, err := cr.NewConn(ctx,
		"https://www.youtube.com/watch?v=EEIk7gwjgIM&absolute_experiments="+
			s.RequiredVar("ui.VideoCUJ.ytExperiments"),
		cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open youtube: ", err)
	}
	defer ytConn.Close()

	var ytWinID int
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get all window: ", err)
	} else if len(all) != 2 {
		s.Fatalf("Expect 2 windows, got %d", len(all))
	} else {
		if webWinID == all[0].ID {
			ytWinID = all[1].ID
		} else {
			ytWinID = all[0].ID
		}
	}

	// Wait for <video> tag to show up.
	if err := webutil.WaitForYoutubeVideo(ctx, ytConn, 0); err != nil {
		s.Fatal("Failed to wait for video element: ", err)
	}

	// Hold alt a bit then tab to show the window cycle list.
	altTab := func() error {
		if err := kb.AccelPress(ctx, "Alt"); err != nil {
			return errors.Wrap(err, "failed to press alt")
		}
		defer kb.AccelRelease(ctx, "Alt")
		if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		if err := kb.Accel(ctx, "Tab"); err != nil {
			return errors.Wrap(err, "failed to type tab")
		}
		if err := testing.Sleep(ctx, time.Second); err != nil {
			return errors.Wrap(err, "failed to wait")
		}
		return nil
	}

	getYtElemBounds := func(sel string) (coords.Rect, error) {
		var bounds coords.Rect
		if err := ytConn.Eval(ctx, fmt.Sprintf(
			`(function() {
				  var b = document.querySelector(%q).getBoundingClientRect();
					return {
						'left': Math.round(b.left),
						'top': Math.round(b.top),
						'width': Math.round(b.width),
						'height': Math.round(b.height),
					};
				})()`,
			sel), &bounds); err != nil {
			return coords.Rect{}, errors.Wrapf(err, "failed to get bounds for selector %q", sel)
		}

		return bounds, nil
	}

	// Returns bounds of the element when the element does not change its bounds
	// and is at the top (a tap/click should reach it).
	getTappableYtElemBounds := func(sel string) (coords.Rect, error) {
		var bounds coords.Rect

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if newBounds, err := getYtElemBounds(sel); err != nil {
				return err
			} else if newBounds != bounds {
				bounds = newBounds
				return errors.New("bounds are changing")
			}

			if bounds.Width == 0 || bounds.Height == 0 {
				return errors.Errorf("bad bound for selector %q", sel)
			}

			var atTop bool
			if err := ytConn.Eval(ctx, fmt.Sprintf(
				`(function() {
						var sel = document.querySelector(%q);
						var el = document.elementFromPoint(%d, %d);
						return sel.contains(el);
					})()`,
				sel, bounds.CenterPoint().X, bounds.CenterPoint().Y),
				&atTop); err != nil {
				return errors.Wrapf(err, "failed to check at top of selector %q", sel)
			}
			if !atTop {
				return errors.Errorf("selector %q is not at top", sel)
			}

			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return coords.Rect{}, err
		}

		return bounds, nil
	}

	tapYtElem := func(sel string) error {
		bounds, err := getTappableYtElemBounds(sel)
		if err != nil {
			return err
		}

		all, err := chromeui.FindAll(ctx, tconn,
			chromeui.FindParams{
				ClassName: "WebContentsViewAura",
				Role:      chromeui.RoleTypeWindow})
		if err != nil {
			return errors.Wrap(err, "failed to find WebContentsViewAura node")
		}
		defer all.Release(ctx)

		var ytWeb *chromeui.Node
		for _, n := range all {
			if strings.Contains(n.Name, "YouTube") {
				ytWeb = n
				break
			}
		}
		if ytWeb == nil {
			return errors.Wrap(err, "failed to find YouTube WebContentsViewAura node")
		}

		tapPoint := bounds.CenterPoint().Add(ytWeb.Location.TopLeft())
		if err := pointer.Click(ctx, pc, tapPoint); err != nil {
			return errors.Wrapf(err, "failed to tap selector %q", sel)
		}
		return nil
	}

	tapFullscreenButton := func() error {
		if err := tapYtElem(`.ytp-fullscreen-button`); err != nil {
			// The failure could be caused by promotion banner covering the button.
			// It could happen in small screen devices. Attempt to dismiss the banner.
			// Ignore the error since the banner might not be there.
			if err := tapYtElem("ytd-button-renderer#dismiss-button"); err != nil {
				s.Log("Failed to dismiss banner: ", err)
			}

			// Attempt to dismiss floating surveys that could  cover the bottom-right
			// and ignore the errors since the survey could not be there..
			if err := tapYtElem("button[aria-label='Dismiss']"); err != nil {
				s.Log("Failed to dismiss survey: ", err)
			}

			// Tap the video to pause it to ensure the fullscreen button showing up.
			if err := tapYtElem(`video`); err != nil {
				return errors.Wrap(err, "failed to tap video to pause it")
			}

			// Tap fullscreen button again.
			if err := tapYtElem(`.ytp-fullscreen-button`); err != nil {
				return errors.Wrap(err, "failed to tap fullscreen button")
			}

			if err := tapYtElem(`video`); err != nil {
				return errors.Wrap(err, "failed to tap video to resume it")
			}
		}

		return nil
	}

	enterFullscreen := func() error {
		if ytWin, err := ash.GetWindow(ctx, tconn, ytWinID); err != nil {
			return errors.Wrap(err, "failed to get youtube window")
		} else if ytWin.State == ash.WindowStateFullscreen {
			return errors.New("alreay in fullscreen")
		}

		if err := tapFullscreenButton(); err != nil {
			return err
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == ytWinID && w.State == ash.WindowStateFullscreen
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for fullscreen")
		}

		return nil
	}

	if err := ash.HideAllNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide ash notification: ", err)
	}

	s.Log("Make video fullscreen")
	if err := enterFullscreen(); err != nil {
		s.Fatal("Failed to enter fullscreen: ", err)
	}

	var configs []cuj.MetricConfig
	if tabletMode {
		configs = append(configs,
			cuj.NewLatencyMetricConfig("Ash.DragWindowFromShelf.PresentationTime"),
			cuj.NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Enter.TabletMode"),
			cuj.NewSmoothnessMetricConfig("Ash.Overview.AnimationSmoothness.Exit.TabletMode"),
		)
	} else {
		configs = append(configs,
			cuj.NewSmoothnessMetricConfig("Ash.WindowCycleView.AnimationSmoothness.Container"),
		)
	}
	recorder, err := cuj.NewRecorder(ctx, tconn, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed waiting for CPU to become idle: ", err)
	}

	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.startSmoothnessTracking)()`, nil); err != nil {
		s.Fatal("Failed to start display smoothness tracking: ", err)
	}

	if err = recorder.Run(ctx, tconn, func() error {
		s.Log("Switch away from fullscreen video")
		if tabletMode {
			if err := tapFullscreenButton(); err != nil {
				return errors.Wrap(err, "failed to tap fullscreen button")
			}

			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == ytWinID && w.State != ash.WindowStateFullscreen
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to wait fullscreen exit")
			}

			if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
				return errors.Wrap(err, "failed to DragToShowOverview")
			}

			w, err := ash.GetWindow(ctx, tconn, webWinID)
			if err != nil {
				return errors.Wrap(err, "failed to find the other window: ")
			}

			if err := pointer.Click(ctx, pc, w.OverviewInfo.Bounds.CenterPoint()); err != nil {
				return errors.Wrap(err, "failed to tap the other window's overview item")
			}
		} else {
			if err := altTab(); err != nil {
				return errors.Wrap(err, "failed to alt-tab")
			}
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == ytWinID && !w.IsActive
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait youtube window deactivate")
		}

		s.Log("Switch back to fullscreen video")
		if tabletMode {
			if err := ash.DragToShowOverview(ctx, tsw.Width(), tsw.Height(), stw, tconn); err != nil {
				return errors.Wrap(err, "failed to DragToShowOverview")
			}

			ytWin, err := ash.GetWindow(ctx, tconn, ytWinID)
			if err != nil {
				return errors.Wrap(err, "failed to get youtube window")
			}

			if err := pointer.Click(ctx, pc, ytWin.OverviewInfo.Bounds.CenterPoint()); err != nil {
				return errors.Wrap(err, "failed to select youtube window")
			}

			if err := enterFullscreen(); err != nil {
				return errors.Wrap(err, "failed to make video fullscreen")
			}
		} else {
			if err := altTab(); err != nil {
				return errors.Wrap(err, "failed to alt-tab")
			}
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == ytWinID && w.IsActive && w.State == ash.WindowStateFullscreen
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait active fullscreen youtube window")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed: ", err)
	}

	// Calculate display smoothness.
	s.Log("Get display smoothness")
	var ds float64
	if err := tconn.EvalPromise(ctx,
		`tast.promisify(chrome.autotestPrivate.stopSmoothnessTracking)()`, &ds); err != nil {
		s.Fatal("Failed to stop display smoothness tracking: ", err)
	}
	s.Log("Display smoothness: ", ds)

	// Get video smoothness.
	s.Log("Get video smoothness")
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

	// Before recording the metrics, check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
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

	if err = recorder.Record(pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
