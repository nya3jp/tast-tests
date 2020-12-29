// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/gmail"
	"chromiumos/tast/local/bundles/cros/ui/videocuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC05S1YoutubeVideoCUJ,
		Desc:         "Measures the smoothess of switch between full screen YouTube video and a tab",
		Contacts:     []string{"xiyuan@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      6 * time.Minute,
		Pre:          cuj.LoginKeepState(),
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
		},
	})
}

func TC05S1YoutubeVideoCUJ(ctx context.Context, s *testing.State) {
	tabletMode := false

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	cr := s.PreValue().(cuj.PreKeepData).Chrome
	a := s.PreValue().(cuj.PreKeepData).ARC
	loginTime := s.PreValue().(cuj.PreKeepData).LoginTime
	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create TabCrashChecker: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	pc := pointer.NewMouseController(tconn)
	defer pc.Close()

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	const (
		gmailPkg   = "com.google.android.gm"
		youtubePkg = "com.google.android.youtube"
	)

	s.Log("Check installed packages")
	pkgs, err := a.InstalledPackages(ctx)
	if err != nil {
		s.Fatal("Failed to list the installed packages: ", err)
	}

	for _, pkgName := range []string{gmailPkg, youtubePkg} {
		if _, ok := pkgs[pkgName]; ok {
			s.Logf("%s is already installed", pkgName)
			continue
		}
		s.Log("Installing ", pkgName)
		installCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		if err = playstore.InstallApp(installCtx, a, d, pkgName, -1); err != nil {
			cancel()
			s.Fatalf("Failed to install %s: %v", pkgName, err)
		}
		if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
			s.Fatal("Failed to close Play Store: ", err)
		}
		cancel()
	}

	s.Log("Open Gmail app")
	if _, err := gmail.New(ctx, tconn, d); err != nil {
		s.Fatal("Failed to open Gmail: ", err)
	}

	var gmWinID int
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get all window: ", err)
	} else if len(all) != 1 {
		s.Fatalf("Expect 1 window, got %d", len(all))
	} else {
		gmWinID = all[0].ID
	}

	s.Log("Start to get browser start time")
	browserStartTime, err := cuj.GetOpenBrowserStartTime(ctx, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	s.Log("Open youtube Web")
	ytConn, err := cr.NewConn(ctx,
		"https://www.youtube.com/watch?v=b3wcQqINmE4",
		cdputil.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open youtube: ", err)
	}
	defer ytConn.Close()
	defer ytConn.CloseTarget(ctx)

	var ytWinID int
	if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
		s.Fatal("Failed to get all window: ", err)
	} else if len(all) != 2 {
		s.Fatalf("Expect 2 windows, got %d", len(all))
	} else {
		if gmWinID == all[0].ID {
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
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for fullscreen")
		}

		return nil
	}

	switchQuality := func(resolution string) error {
		const waitTime = time.Second * 3
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			testing.ContextLog(ctx, "Click 'Settings'")
			if err := cuj.WaitAndClick(ctx, tconn, chromeui.FindParams{
				Role: chromeui.RoleTypePopUpButton,
				Name: "Settings"}, waitTime); err != nil {
				return errors.Wrap(err, "failed to click 'Settings'")
			}

			testing.Sleep(ctx, waitTime)

			testing.ContextLog(ctx, "Click 'Quality'")
			if err := cuj.WaitAndClick(ctx, tconn, chromeui.FindParams{
				Attributes: map[string]interface{}{"name": regexp.MustCompile(`^Quality`)},
				Role:       chromeui.RoleTypeMenuItem}, waitTime); err != nil {
				return errors.Wrap(err, "failed to click 'Quality'")
			}

			testing.Sleep(ctx, waitTime)

			testing.ContextLogf(ctx, "Click %q", resolution)
			if err := cuj.WaitAndClick(ctx, tconn, chromeui.FindParams{
				Attributes: map[string]interface{}{
					"name": regexp.MustCompile(fmt.Sprintf("^%s", resolution)),
				},
				Role: chromeui.RoleTypeMenuItemRadio}, waitTime); err != nil {
				return errors.Wrapf(err, "failed to click %q", resolution)
			}

			return nil
		}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: time.Minute}); err != nil {
			return errors.Wrap(err, "failed to switch Quality")
		}
		if err := testing.Sleep(ctx, waitTime); err != nil {
			return errors.Wrap(err, "failed to sleep")
		}
		s.Log("Verify youtube is ready to play")
		if err := waitForYoutubeReadyState(ctx, ytConn); err != nil {
			return errors.Wrap(err, "failed to wait for Youtube ready state")
		}

		return nil
	}

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide ash notification: ", err)
	}

	s.Log("Make video fullscreen")
	if err := enterFullscreen(); err != nil {
		s.Fatal("Failed to enter fullscreen: ", err)
	}

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 20)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	} else {
		defer setBatteryNormal(ctx)
	}

	if err = recorder.Run(ctx, func(ctx context.Context) error {
		testResolutions := []string{"2160p", "1080p", "480p"}
		for _, resolution := range testResolutions {
			testing.Sleep(ctx, time.Second*3)

			s.Log("Switch to the new quality")
			if err := switchQuality(resolution); err != nil {
				return errors.Wrap(err, "failed to switch quality; resolution = "+resolution)
			}

			s.Log("Switch away from fullscreen video")
			if err := altTab(); err != nil {
				return errors.Wrap(err, "failed to alt-tab")
			}

			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == ytWinID && !w.IsActive && !w.IsAnimating
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to wait youtube window deactivate")
			}

			s.Log("Switch back to fullscreen video")
			if err := altTab(); err != nil {
				return errors.Wrap(err, "failed to alt-tab")
			}

			if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
				return w.ID == ytWinID && w.IsActive && w.State == ash.WindowStateFullscreen
			}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
				return errors.Wrap(err, "failed to wait active fullscreen youtube window")
			}
		}

		if err := videocuj.DoYoutubeCUJ(ctx, tconn, a, d); err != nil {
			return errors.Wrap(err, "failed to do YouTube cuj")
		}

		return nil
	}); err != nil {
		s.Fatal("Failed: ", err)
	}

	s.Log("Press Enter")
	if err := kb.Accel(ctx, "Enter"); err != nil {
		s.Fatal("Failed to type enter: ", err)
	}

	// Before recording the metrics, check if there is any tab crashed.
	if err := tabChecker.Check(ctx); err != nil {
		s.Fatal("Tab renderer crashed: ", err)
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "User.LoginTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(loginTime))
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime))

	if err = recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// waitForYoutubeReadyState does wait youtube video ready state then return.
func waitForYoutubeReadyState(ctx context.Context, conn *chrome.Conn) error {
	// VideoPlayer represents the main <video> node in youtube page.
	const VideoPlayer = "#movie_player > div.html5-video-container > video"
	queryCode := fmt.Sprintf("new Promise((resolve, reject) => { let video = document.querySelector(%q); resolve(video.readyState === 4 && video.buffered.length > 0); });", VideoPlayer)

	// Wait for element to appear.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var pageState bool
		if err := conn.EvalPromise(ctx, queryCode, &pageState); err != nil {
			return err
		}
		if pageState {
			return nil
		}
		return errors.New("failed to wait for ready state")
	}, &testing.PollOptions{Interval: time.Second, Timeout: 40 * time.Second}); err != nil {
		return err
	}
	return nil
}
