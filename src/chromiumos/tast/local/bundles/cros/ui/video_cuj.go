// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/pointer"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type videoCUJTestParam struct {
	bt      browser.Type
	tablet  bool
	tracing bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the smoothess of switch between full screen video and a tab/app",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild", "group:cuj"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      45 * time.Minute,
		Vars: []string{
			"mute",
		},
		VarDeps: []string{
			"ui.VideoCUJ.ytExperiments",
		},
		Params: []testing.Param{{
			Name:    "clamshell",
			Fixture: "loggedInToCUJUser",
			Val: videoCUJTestParam{
				bt: browser.TypeAsh,
			},
		}, {
			Name:    "clamshell_trace",
			Fixture: "loggedInToCUJUser",
			Val: videoCUJTestParam{
				bt:      browser.TypeAsh,
				tracing: true,
			},
		}, {
			Name:    "tablet",
			Fixture: "loggedInToCUJUser",
			Val: videoCUJTestParam{
				bt:     browser.TypeAsh,
				tablet: true,
			},
		}, {
			Name:    "lacros",
			Fixture: "loggedInToCUJUserLacros",
			Val: videoCUJTestParam{
				bt: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}, {
			Name:    "lacros_tablet",
			Fixture: "loggedInToCUJUserLacros",
			Val: videoCUJTestParam{
				bt:     browser.TypeLacros,
				tablet: true,
			},
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func VideoCUJ(ctx context.Context, s *testing.State) {
	// PollOptions to allow blink layout to catch up. Interval at 5s is an
	// arbitrary long interval for slow DUTs where blink layout could stay
	// stale for a while (we have seen 3s on grunt in local test) and the default
	// 100ms interval could take the intermediate layout as the stabilized final
	// one.
	layoutPollOptions := testing.PollOptions{
		Interval: 5 * time.Second,
		Timeout:  20 * time.Second,
	}

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	testParam := s.Param().(videoCUJTestParam)

	var cr *chrome.Chrome
	var cs ash.ConnSource

	if testParam.bt == browser.TypeAsh {
		cr = s.FixtValue().(chrome.HasChrome).Chrome()
		cs = cr
	} else {
		var l *lacros.Lacros
		var err error
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), testParam.bt)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		defer lacros.CloseLacros(ctx, l)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	if _, ok := s.Var("mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute audio: ", err)
		}
		defer crastestclient.Unmute(closeCtx)
	}

	tabletMode := testParam.tablet
	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to ensure tablet/clamshell mode: ", err)
	}
	defer cleanup(closeCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	var pc pointer.Context
	var tsw *input.TouchscreenEventWriter
	var stw *input.SingleTouchEventWriter
	if tabletMode {
		var err error
		pc, err = pointer.NewTouch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to create a touch controller")
		}

		tsw, _, err = touch.NewTouchscreenAndConverter(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to access to the touchscreen: ", err)
		}
		defer tsw.Close()

		stw, err = tsw.NewSingleTouchWriter()
		if err != nil {
			s.Fatal("Failed to create the single touch writer: ", err)
		}
	} else {
		pc = pointer.NewMouse(tconn)
	}
	defer pc.Close()

	ac := uiauto.New(tconn)

	configs := []cuj.MetricConfig{
		cuj.NewCustomMetricConfig(
			"Ash.Smoothness.PercentDroppedFrames_1sWindow", "percent",
			perf.SmallerIsBetter, []int64{50, 80}),
		cuj.NewCustomMetricConfig(
			"Browser.Responsiveness.JankyIntervalsPerThirtySeconds3", "janks",
			perf.SmallerIsBetter, []int64{0, 3}),
	}
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
	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.RecorderOptions{}, configs...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	if testParam.tracing {
		recorder.EnableTracing(s.OutDir())
	}
	defer recorder.Close(closeCtx)

	webConn, err := cs.NewConn(ctx, ui.PerftestURL)
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
	ytConn, err := cs.NewConn(ctx,
		"https://www.youtube.com/watch?v=by_xCK2Jo5c&absolute_experiments="+
			s.RequiredVar("ui.VideoCUJ.ytExperiments"),
		browser.WithNewWindow())
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
		}, &layoutPollOptions); err != nil {
			return coords.Rect{}, err
		}

		return bounds, nil
	}

	tapYtElem := func(sel string) error {
		bounds, err := getTappableYtElemBounds(sel)
		if err != nil {
			return err
		}

		ytWeb := nodewith.ClassName("WebContentsViewAura").Role(role.Window).NameContaining("YouTube")
		location, err := ac.Location(ctx, ytWeb)
		if err != nil {
			return errors.Wrap(err, "failed to get YouTube WebContentsViewAura location")
		}

		tapPoint := bounds.CenterPoint().Add(location.TopLeft())
		if err := pc.ClickAt(tapPoint)(ctx); err != nil {
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
			// and ignore the errors since the survey could not be there.
			if err := tapYtElem("button[aria-label='Dismiss']"); err != nil {
				s.Log("Failed to dismiss survey: ", err)
			}

			// Attempt to dismiss webfe served message box and ignore the errors
			// since the message div could not be there.
			if err := tapYtElem(".webfe-served-box"); err != nil {
				s.Log("Failed to dismiss webfe served message: ", err)
			}

			// Tap the video to pause it to ensure the fullscreen button showing up.
			if err := tapYtElem(`video`); err != nil {
				return errors.Wrap(err, "failed to tap video to pause it")
			}

			// Wait for the button animation to finish. Otherwise, it is not tappable
			// even though it has stable bounds and is at the top.
			if err := testing.Sleep(ctx, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to wait for button animaiton")
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

		// Wait for the 'video' element to be updated to fullscreen. This is needed
		// because blink layout is updated asynchronously with the browser window
		// bounds change. 'video' element is considered fullscreen when either its
		// width or its height matches the screen width or height. This is because
		// 'video' element is resized to keep video aspect ratio and not always
		// filling the screen. The allowed difference of width/height is 2 because
		// the difference is about 0.999 on "nocturne" and arbitrarily use 2 to
		// handle that edge case.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			var fullscreen bool
			if err := ytConn.Eval(ctx,
				`(function() {
						var b = document.querySelector('video').getBoundingClientRect();
						return Math.abs(b.width -  window.screen.width) < 2 ||
						       Math.abs(b.height - window.screen.height) < 2;
					})()`,
				&fullscreen); err != nil {
				return errors.Wrap(err, "failed to check video size")
			}
			if !fullscreen {
				return errors.New("video is not fullscreen")
			}

			return nil
		}, &layoutPollOptions); err != nil {
			return errors.Wrap(err, "failed to wait for blink layout")
		}

		// Wait for the browser window to be in fullscreen state.
		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == ytWinID && w.State == ash.WindowStateFullscreen
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for fullscreen")
		}

		return nil
	}

	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close ash notification: ", err)
	}

	s.Log("Make video fullscreen")
	if err := enterFullscreen(); err != nil {
		s.Fatal("Failed to enter fullscreen: ", err)
	}

	if err := recorder.RunFor(ctx, func(ctx context.Context) error {
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

			if err := ash.DragToShowOverview(ctx, tsw, stw, tconn); err != nil {
				return errors.Wrap(err, "failed to DragToShowOverview")
			}

			w, err := ash.GetWindow(ctx, tconn, webWinID)
			if err != nil {
				return errors.Wrap(err, "failed to find the other window")
			}

			if err := pc.ClickAt(w.OverviewInfo.Bounds.CenterPoint())(ctx); err != nil {
				return errors.Wrap(err, "failed to tap the other window's overview item")
			}
		} else {
			if err := altTab(); err != nil {
				return errors.Wrap(err, "failed to alt-tab")
			}
		}

		if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
			return w.ID == ytWinID && !w.IsActive && !w.IsAnimating
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait youtube window deactivate")
		}

		s.Log("Switch back to fullscreen video")
		if tabletMode {
			if err := ash.DragToShowOverview(ctx, tsw, stw, tconn); err != nil {
				return errors.Wrap(err, "failed to DragToShowOverview")
			}

			ytWin, err := ash.GetWindow(ctx, tconn, ytWinID)
			if err != nil {
				return errors.Wrap(err, "failed to get youtube window")
			}

			if err := pc.ClickAt(ytWin.OverviewInfo.Bounds.CenterPoint())(ctx); err != nil {
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
	}, 30*time.Minute); err != nil {
		s.Fatal("Failed: ", err)
	}

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
		Name:      "VideoCUJ.VideoSmoothness." + metricSuffix,
		Unit:      "percent",
		Direction: perf.BiggerIsBetter,
	}, vs)

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
