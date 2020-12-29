// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videocuj contains the test code for VideoCUJ.
package videocuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

const (
	// YoutubeWeb indicates to test against Youtube web
	YoutubeWeb = "YoutubeWeb"
	// YoutubeApp indicates to test against Youtube app
	YoutubeApp = "YoutubeApp"
)

// VideoApp declares video operation.
type VideoApp interface {
	// OpenAndPlayVideo opens a video.
	OpenAndPlayVideo(ctx context.Context) error
	// EnterFullscreen switches video to fullscreen.
	EnterFullscreen(ctx context.Context) error
	// PauseAndPlayVideo verifies video playback.
	PauseAndPlayVideo(ctx context.Context) error
	// Close closes the resources related to video.
	Close(ctx context.Context)
}

// Video struct defined video src for testing
type Video struct {
	url     string
	quality string
}

var basicVideoSrc = []Video{
	{"https://www.youtube.com/watch?v=suWsd372pQE", "1080p"},
	{"https://www.youtube.com/watch?v=b3wcQqINmE4", "720p60"},
	{"https://www.youtube.com/watch?v=b3wcQqINmE4", "1080p60"},
}

var plusVideoSrc = []Video{
	{"https://www.youtube.com/watch?v=b3wcQqINmE4", "2160p60"},
}

// Run runs the VideoCUJ test.
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, a *arc.ARC, appName string, tabletMode bool, tier cuj.Tier, extendedDisplay bool) {
	s.Logf("Run app appName: %s tabletMode: %t, extendedDisplay: %t", appName, tabletMode, extendedDisplay)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if appName == YoutubeApp {
		if err := installYoutubeApp(ctx, tconn, a); err != nil {
			s.Fatalf("Failed to install %s: %v", youtubePkg, err)
		}
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

	var uiHandler cuj.UIActionHandler
	if tabletMode {
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer uiHandler.Close()

	ui := uiauto.New(tconn)

	openGmailWeb := func(ctx context.Context) (*chrome.Conn, error) {
		conn, err := uiHandler.NewChromeTab(ctx, cr, "https://mail.google.com", true)
		if err != nil {
			return conn, errors.Wrap(err, "failed to open gmail web page")
		}
		if err := webutil.WaitForQuiescence(ctx, conn, 2*time.Minute); err != nil {
			return conn, errors.Wrap(err, "failed ailed to wait for page to finish loading")
		}

		gotItPrompt := nodewith.Name("Got it").Role(role.Button)
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(gotItPrompt); err == nil {
			if err := uiauto.Combine("click 'Got it' prompt",
				// Immediately clicking the 'Got it' button sometimes doesn't work. Sleep 2 seconds.
				ui.Sleep(2*time.Second),
				uiHandler.Click(gotItPrompt),
			)(ctx); err != nil {
				testing.ContextLog(ctx, "There's no 'Got it' prompt")
			}
		}

		return conn, nil
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(ctx)

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge: ", err)
	}
	defer setBatteryNormal(ctx)

	s.Log("Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	videoSrc := basicVideoSrc
	if tier == cuj.Plus {
		videoSrc = plusVideoSrc
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	runScenario := func(ctx context.Context, video Video) {
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			var videoApp VideoApp
			switch appName {
			case YoutubeWeb:
				videoApp = NewYtWeb(cr, tconn, kb, video, extendedDisplay, ui, uiHandler)
			case YoutubeApp:
				videoApp = NewYtApp(tconn, kb, a, d, video)
			}

			if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
				s.Fatalf("Failed to open %s: %v", appName, err)
			}
			defer videoApp.Close(ctx)

			// Play video at fullscreen.
			if err := videoApp.EnterFullscreen(ctx); err != nil {
				s.Fatal("Failed to play video in fullscreen: ", err)
			}

			// Delay time between fullscreen video and open Gmail web.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				s.Fatal("Failed to sleep: ", err)
			}

			// Open Gmail web.
			s.Log("Open Gmail web")
			gConn, err := openGmailWeb(ctx)
			if err != nil {
				s.Fatal("Failed to open Gmail website: ", err)
			}
			defer gConn.Close()
			defer gConn.CloseTarget(ctx)

			if appName == YoutubeApp {
				if err = checkYoutubeAppPIP(ctx, tconn); err != nil {
					s.Fatal("Youtube App smaller video preview window is not shows up : ", err)
				}
			}

			// Switch back to video playing.
			if tabletMode && appName == YoutubeApp {
				if err := uiHandler.SwitchToAppWindow("YouTube")(ctx); err != nil {
					return errors.Wrap(err, "failed to click app from Hotseat")
				}
				if err := kb.Accel(ctx, "F4"); err != nil {
					return errors.Wrap(err, "failed to type the tab key")
				}
			} else {
				if err := uiHandler.SwitchWindow()(ctx); err != nil {
					return errors.Wrap(err, "failed to switch back to video playing")
				}
			}

			// Pause and reuse video playback.
			if err := videoApp.PauseAndPlayVideo(ctx); err != nil {
				return errors.Wrap(err, "failed to pause and play video")
			}

			// Before recording the metrics, check if there is any tab crashed.
			if err := tabChecker.Check(ctx); err != nil {
				return errors.Wrap(err, "tab renderer crashed")
			}
			return nil
		}); err != nil {
			s.Fatal("Failed on run recorder: ", err)
		}
	}
	for index := 0; index < len(videoSrc); index++ {
		runScenario(ctx, videoSrc[index])
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, float64(browserStartTime.Milliseconds()))

	if appName == YoutubeApp {
		pv.Set(perf.Metric{
			Name:      "Apps.StartTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(appStartTime))
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}

func waitWindowStateFullscreen(ctx context.Context, tconn *chrome.TestConn, winID int) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.ID == winID && w.State == ash.WindowStateFullscreen
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for fullscreen")
	}
	return nil
}

func getWindowID(ctx context.Context, tconn *chrome.TestConn) (int, error) {
	all, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get all windows")
	}
	if len(all) != 1 {
		return 0, errors.Wrapf(err, "expect 1 windoe, got %d", len(all))
	}
	return all[0].ID, nil
}

func installYoutubeApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	closeCtx := ctx
	installCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	device, err := a.NewUIDevice(installCtx)
	if err != nil {
		return errors.Wrap(err, "failed to set up ARC device")
	}
	defer device.Close(closeCtx)

	installErr := playstore.InstallApp(installCtx, a, device, youtubePkg, -1)

	if err := apps.Close(closeCtx, tconn, apps.PlayStore.ID); err != nil {
		return errors.Wrap(err, "failed to close Play Store")
	}

	return installErr
}
