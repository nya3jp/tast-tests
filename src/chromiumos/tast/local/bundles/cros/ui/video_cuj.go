// Copyright 2020 The ChromiumOS Authors
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/cuj/inputsimulations"
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
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type videoCUJTestParam struct {
	bt     browser.Type
	tablet bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         VideoCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of a critical user journey for YouTube web",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		// TODO(b/234063928): Remove crosbolt attributes when VideoCUJ runs stably on suite cuj.
		Attr:         []string{"group:cuj"},
		SoftwareDeps: []string{"chrome", "arc"},
		Data:         []string{cujrecorder.SystemTraceConfigFile},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      45 * time.Minute,
		Vars: []string{
			"mute",
			"record",
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

	// Determines the overall test duration.
	const testDuration = 10 * time.Minute

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var cs ash.ConnSource
	var bTconn *chrome.TestConn
	switch testParam.bt {
	case browser.TypeLacros:
		// Launch lacros.
		l, err := lacros.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch lacros: ", err)
		}
		defer l.Close(ctx)
		cs = l

		if bTconn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	case browser.TypeAsh:
		cs = cr
		bTconn = tconn
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

	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	if err := recorder.AddCommonMetrics(tconn, bTconn); err != nil {
		s.Fatal("Failed to add common metrics to recorder: ", err)
	}

	recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	defer recorder.Close(closeCtx)

	if _, ok := s.Var("record"); ok {
		if err := recorder.AddScreenRecorder(ctx, tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

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

	// Dismiss any potential popup notification.
	uiLongWait := ac.WithTimeout(15 * time.Second)
	bubble := nodewith.ClassName("PermissionPromptBubbleView").First()
	allow := nodewith.Name("Allow").Role(role.Button).Ancestor(bubble)
	// Check and grant permissions.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Long wait for permission bubble and break poll loop when it times out.
		if uiLongWait.WaitUntilExists(bubble)(ctx) != nil {
			return nil
		}
		if err := pc.Click(allow)(ctx); err != nil {
			return errors.Wrap(err, "failed to click the Allow button")
		}
		return errors.New("granting permissions")
	}, &testing.PollOptions{Interval: time.Second, Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to grant permissions: ", err)
	}

	// Check the paused attribute on the HTML video element on the
	// Youtube webpage to determine its current playing state.
	isYoutubeVideoPlaying := func() (bool, error) {
		const script = `document.querySelector("video").paused`
		var isPaused bool
		if err := ytConn.Eval(ctx, script, &isPaused); err != nil {
			return false, err
		}
		return !isPaused, nil
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

	s.Log("Run test for ", testDuration)
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

		// Adjust volume by pressing and holding the up and down arrow.
		if err := inputsimulations.RepeatKeyHold(ctx, kb, "Down", 2*time.Second, time.Second, 1); err != nil {
			return errors.Wrap(err, "failed to lower volume with down arrow")
		}
		if err := inputsimulations.RepeatKeyHold(ctx, kb, "Up", 2*time.Second, time.Second, 1); err != nil {
			return errors.Wrap(err, "failed to raise volume with up arrow")
		}

		// Press the spacebar 4 times, with a 3 second delay between presses.
		// Since the video is playing before the spacebar is pressed,
		// the state of the video will be: paused -> playing -> paused -> playing,
		// with the state of the video changing every 5 seconds.
		if err := inputsimulations.RepeatKeyPress(ctx, kb, "Space", 3*time.Second, 4); err != nil {
			return errors.Wrap(err, "failed to pause and play the video")
		}

		// Ensure the video is playing after toggling with the spacebar.
		if isPlaying, err := isYoutubeVideoPlaying(); err != nil {
			return errors.Wrap(err, "failed to check video playing status")
		} else if !isPlaying {
			return errors.New("youtube video is paused, video is expected to be playing")
		}

		// Simulate user passively watching video.
		if err := testing.Sleep(ctx, 30*time.Second); err != nil {
			return errors.Wrap(err, "failed to idle while watching video")
		}

		return nil
	}, testDuration); err != nil {
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
