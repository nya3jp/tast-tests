// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videocuj contains the test code for VideoCUJ.
package videocuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/playstore"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/pointer"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
	"chromiumos/tast/testing"
)

const (
	//YoutubeWeb indicates to test against Youtube web
	YoutubeWeb = "YoutubeWeb"
	//YoutubeApp indicates to test against Youtube app
	YoutubeApp = "YoutubeApp"
	//NetflixWeb indicates to test against Netflix web
	NetflixWeb = "NetflixWeb"
)

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

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	if appName == YoutubeApp {
		// Install Youtube App.
		device, err := a.NewUIDevice(ctx)
		if err != nil {
			s.Fatal("Failed to set up ARC device: ", err)
		}
		defer device.Close(ctx)

		if err := playstore.InstallApp(ctx, a, device, youtubePkg, -1); err != nil {
			s.Fatalf("Failed to install %s: %v", youtubePkg, err)
		}
		if err := apps.Close(ctx, tconn, apps.PlayStore.ID); err != nil {
			s.Error("Failed to close Play Store: ", err)
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

	pc := pointer.NewMouseController(tconn)
	defer pc.Close()

	if extendedDisplay && tabletMode {
		if err := unsetMirrorDisplay(ctx, s, tconn); err != nil {
			s.Fatal("Failed to unset mirror display: ", err)
		}
	}

	openGmailWeb := func(ctx context.Context) (*chrome.Conn, error) {
		const url = "https://mail.google.com"
		conn, err := cr.NewConn(ctx, url, cdputil.WithNewWindow())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open %s", url)
		}
		if err := webutil.WaitForQuiescence(ctx, conn, time.Minute*2); err != nil {
			return nil, errors.Wrap(err, "failed ailed to wait for page to finish loading")
		}
		cuj.WaitAndClick(ctx, tconn, ui.FindParams{Name: "Got it", Role: ui.RoleTypeButton}, time.Second)
		return conn, nil
	}

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}
	defer recorder.Close(closeCtx)

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

	var appStartTime int64

	repeatTimes := 1
	videoSrc := basicVideoSrc
	netflixPlaybackSettings := "Basic"

	if tier == cuj.Plus {
		videoSrc = plusVideoSrc
		netflixPlaybackSettings = "Plus"
	}
	if appName == YoutubeWeb || appName == YoutubeApp {
		repeatTimes = len(videoSrc)
	}

	var ytConn *chrome.Conn
	var ytAct *arc.Activity

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed to setup ARC and Play Store: ", err)
	}
	defer d.Close(ctx)

	runScenario := func(ctx context.Context, index int) {
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			// Open app and play the video.
			switch appName {
			case YoutubeWeb:
				ytConn, err = openAndPlayYoutubeWeb(ctx, tconn, cr, videoSrc[index])
				if err != nil {
					s.Fatal("Failed to open Youtube web: ", err)
				}
				defer ytConn.Close()
				defer ytConn.CloseTarget(ctx)
			case YoutubeApp:
				ytAct, appStartTime, err = openAndPlayYoutubeApp(ctx, s, tconn, a, d, videoSrc[index], index)
				if err != nil {
					s.Fatal("Failed to open Youtube app: ", err)
				}
				defer ytAct.Close()
				defer ytAct.Stop(ctx, tconn)
			case NetflixWeb:
				if _, err := openAndPlayNetflixWeb(ctx, s, tconn, cr, kb, netflixPlaybackSettings, extendedDisplay); err != nil {
					s.Fatal("Failed to open Netflix web: ", err)
				}
			}

			var appWinID int
			if all, err := ash.GetAllWindows(ctx, tconn); err != nil {
				s.Fatal("Failed to get all window: ", err)
			} else if len(all) != 1 {
				s.Fatalf("Expect 1 window, got %d", len(all))
			} else {
				appWinID = all[0].ID
			}
			// Play video at full screen.
			switch appName {
			case YoutubeWeb:
				if err := enterYoutubeWebFullscreen(ctx, tconn, ytConn, appWinID); err != nil {
					s.Fatal("Failed to play Youtube web in fullscreen: ", err)
				}
			case YoutubeApp:
				if err := enterYoutubeAppFullscreen(ctx, tconn, a, d); err != nil {
					s.Fatal("Failed to play Youtube app in fullscreen: ", err)
				}
			case NetflixWeb:
				if err := enterNetflixWebFullscreen(ctx, tconn, appWinID); err != nil {
					s.Fatal("Failed to play Netflix web in fullscreen: ", err)
				}
			}

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
				const youtubeAppName = "YouTube"
				if _, err := cuj.LaunchAppFromHotseat(ctx, tconn, youtubeAppName); err != nil {
					return errors.Wrap(err, "failed to click app from Hotseat")
				}
				if err := kb.Accel(ctx, "F4"); err != nil {
					return errors.Wrap(err, "failed to type the tab key")
				}
			} else {
				if err := altTab(ctx, s, kb); err != nil {
					return errors.Wrap(err, "failed to alt-tab")
				}
			}
			// Pause and reuse video playback.
			switch appName {
			case YoutubeWeb:
				if err := pauseAndPlayYoutubeWeb(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to pause and play Youtube web")
				}
			case YoutubeApp:
				if err := pauseAndPlayYoutubeApp(ctx, a, d); err != nil {
					return errors.Wrap(err, "failed to pause and play Youtube app")
				}
			case NetflixWeb:
				if err := pauseAndPlayNetflixWeb(ctx, tconn, kb); err != nil {
					return errors.Wrap(err, "failed to pause and play Netflix web")
				}
			}

			if extendedDisplay {
				if err := moveGmailWindow(ctx, s, gConn, kb); err != nil {
					return errors.Wrap(err, "failed to move Gmail window between main display and extended display")
				}
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
	for index := 0; index < repeatTimes; index++ {
		runScenario(ctx, index)
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

// unsetMirrorDisplay disables external display mirror mode for tablet
func unsetMirrorDisplay(ctx context.Context, s *testing.State, tConn *chrome.TestConn) error {
	s.Log("Launch os settins to disable mirror")
	settings, err := ossettings.LaunchAtPage(ctx, tConn, ossettings.Device)
	if err != nil {
		return errors.Wrap(err, "failed to launch Settings: ")
	}

	if err := settings.LaunchDisplay()(ctx); err != nil {
		return errors.Wrap(err, "failed to launch display Page")
	}

	if err := settings.ClickMirrorDisplay()(ctx); err != nil {
		return errors.Wrap(err, "failed to click mirror display")
	}

	if err := settings.Close(ctx); err != nil {
		return errors.Wrap(err, "failed to close Settings")
	}

	return nil
}

// moveGmailWindow switch Gmail to the extended display
func moveGmailWindow(ctx context.Context, s *testing.State, gConn *chrome.Conn, kb *input.KeyboardEventWriter) error {
	s.Log("Move focus to Gmail and move it to extended screen")
	if err := altTab(ctx, s, kb); err != nil {
		return errors.Wrap(err, "failed to alt-tab")
	}

	s.Log("Switch Gmail to the extended display")
	if err := kb.Accel(ctx, "Search+Alt+M"); err != nil {
		return errors.Wrap(err, "failed to switch Gmail to the extended display")
	}

	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return errors.Wrap(err, "failed to wait")
	}

	s.Log("Switch Gmail back to the main display")
	if err := kb.Accel(ctx, "Search+Alt+M"); err != nil {
		return errors.Wrap(err, "failed to switch Gmail back to the main display")
	}

	return nil
}

// altTab hold alt a bit then tab to show the window cycle list.
func altTab(ctx context.Context, s *testing.State, kb *input.KeyboardEventWriter) error {
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
