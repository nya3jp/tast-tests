// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videocuj contains the test code for VideoCUJ.
package videocuj

import (
	"context"
	"path/filepath"
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
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const (
	// YoutubeWeb indicates to test against Youtube web.
	YoutubeWeb = "YoutubeWeb"
	// YoutubeApp indicates to test against Youtube app.
	YoutubeApp = "YoutubeApp"
)

// TestResources holds the cuj test resources passed in from main test case.
type TestResources struct {
	Cr        *chrome.Chrome
	LFixtVal  lacrosfixt.FixtValue
	Tconn     *chrome.TestConn
	A         *arc.ARC
	Kb        *input.KeyboardEventWriter
	UIHandler cuj.UIActionHandler
}

// TestParams holds the cuj test parameters passed in from main test case.
type TestParams struct {
	OutDir          string
	App             string
	TabletMode      bool
	Tier            cuj.Tier
	ExtendedDisplay bool
}

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

// VideoSrc struct defines video src for testing.
type VideoSrc struct {
	URL   string
	Title string
	// Quality is the string that test will look for in youtube
	// "Settings / Quality" menu to change video playback quality.
	Quality string
}

var basicVideoSrc = []VideoSrc{
	{
		cuj.YoutubeGoogleTVVideoURL,
		"Chris Paul | Watch With Me | Google TV",
		"1080p",
	},
	{
		cuj.YoutubeDeveloperKeynoteVideoURL,
		"Developer Keynote (Google I/O '21) - American Sign Language",
		"720p60",
	},
	{
		cuj.YoutubeStadiaGDCVideoURL,
		"Stadia GDC 2019 Gaming Announcement",
		"1080p60",
	},
}

var premiumVideoSrc = []VideoSrc{
	{
		cuj.YoutubeStadiaGDCVideoURL,
		"Stadia GDC 2019 Gaming Announcement",
		"2160p60",
	},
}

// Run runs the VideoCUJ test.
func Run(ctx context.Context, resources TestResources, param TestParams) error {
	var (
		cr              = resources.Cr
		tconn           = resources.Tconn
		a               = resources.A
		kb              = resources.Kb
		uiHandler       = resources.UIHandler
		outDir          = param.OutDir
		appName         = param.App
		tabletMode      = param.TabletMode
		tier            = param.Tier
		extendedDisplay = param.ExtendedDisplay
	)

	testing.ContextLogf(ctx, "Run app appName: %s tabletMode: %t, extendedDisplay: %t", appName, tabletMode, extendedDisplay)

	if appName == YoutubeApp {
		if err := installYoutubeApp(ctx, tconn, a); err != nil {
			return errors.Wrapf(err, "failed to install %s", youtubePkg)
		}
	}

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create TabCrashChecker")
	}

	ui := uiauto.New(tconn)

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	testing.ContextLog(ctx, "Start to get browser start time")
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, resources.LFixtVal != nil)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	// If lacros exists, close lacros finally.
	if l != nil {
		defer l.Close(ctx)
	}

	br := cr.Browser()
	tconns := []*chrome.TestConn{tconn}
	var bTconn *chrome.TestConn
	if resources.LFixtVal != nil {
		br = l.Browser()
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create test API conn")
		}
		tconns = append(tconns, bTconn)
	}

	videoSources := basicVideoSrc
	if tier == cuj.Premium {
		videoSources = premiumVideoSrc
	}

	// Give 5 seconds to clean up device objects connected to UI Automator server resources.
	cleanupDeviceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create new ARC device")
	}
	defer func(ctx context.Context) {
		if d.Alive(ctx) {
			testing.ContextLog(ctx, "UI device is still alive")
			d.Close(ctx)
		}
	}(cleanupDeviceCtx)

	openGmailWeb := func(ctx context.Context) (*chrome.Conn, error) {
		// If there's a lacros browser, bring it to active.
		lacrosWin, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return w.WindowType == ash.WindowTypeLacros
		})
		if err != nil && err != ash.ErrWindowNotFound {
			return nil, errors.Wrap(err, "failed to find lacros window")
		}
		if err == nil {
			if err := lacrosWin.ActivateWindow(ctx, tconn); err != nil {
				return nil, errors.Wrap(err, "failed to activate lacros window")
			}
		}

		conn, err := uiHandler.NewChromeTab(ctx, br, cuj.GmailURL, true)
		if err != nil {
			return conn, errors.Wrap(err, "failed to open gmail web page")
		}
		if err := webutil.WaitForQuiescence(ctx, conn, 2*time.Minute); err != nil {
			return conn, errors.Wrap(err, "failed to wait for gmail page to finish loading")
		}
		// YouTube sometimes pops up a prompt to notice users how to operate YouTube
		// if there're new features. Dismiss prompt if it exist.
		gotItPrompt := nodewith.Name("Got it").Role(role.Button)
		uiauto.IfSuccessThen(
			ui.WaitUntilExists(gotItPrompt),
			uiHandler.ClickUntil(
				gotItPrompt,
				ui.WithTimeout(2*time.Second).WaitUntilGone(gotItPrompt),
			),
		)
		return conn, nil
	}

	// Give 5 seconds to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	options := cuj.NewPerformanceCUJOptions()
	recorder, err := cuj.NewRecorder(ctx, cr, a, options, cuj.MetricConfigs(tconns)...)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanupRecorderCtx)

	for _, videoSource := range videoSources {
		// Repeat the run for different video source.
		if err = recorder.Run(ctx, func(ctx context.Context) (retErr error) {
			var videoApp VideoApp
			switch appName {
			case YoutubeWeb:
				videoApp = NewYtWeb(br, tconn, kb, videoSource, extendedDisplay, ui, uiHandler)
			case YoutubeApp:
				videoApp = NewYtApp(tconn, kb, a, d, videoSource)
			}

			// Give time to cleanup videoApp resources.
			cleanupResourceCtx := ctx
			ctx, cancel = ctxutil.Shorten(ctx, 15*time.Second)
			defer cancel()

			defer func(ctx context.Context) {
				// Make sure to close the arc UI device before calling the function. Otherwise uiautomator might have errors.
				if appName == YoutubeApp && retErr != nil {
					if err := d.Close(ctx); err != nil {
						testing.ContextLog(ctx, "Failed to close ARC UI device: ", err)
					}
					a.DumpUIHierarchyOnError(ctx, filepath.Join(outDir, "arc"), func() bool { return retErr != nil })
				}
				faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
				if appName == YoutubeWeb && resources.LFixtVal != nil {
					// For lacros, leave a new tab to keep the browser alive for further testing.
					if err := cuj.KeepNewTab(ctx, bTconn); err != nil {
						testing.ContextLog(ctx, "Failed to keep new tab: ", err)
					}
				} else {
					videoApp.Close(ctx)
				}
			}(cleanupResourceCtx)

			if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
				return errors.Wrapf(err, "failed to open %s", appName)
			}

			// Play video at fullscreen.
			if err := videoApp.EnterFullscreen(ctx); err != nil {
				return errors.Wrap(err, "failed to play video in fullscreen")
			}

			// Let the video play in fullscreen for some time.
			if err := testing.Sleep(ctx, time.Second); err != nil {
				return errors.Wrap(err, "failed to sleep")
			}

			// Open Gmail web.
			testing.ContextLog(ctx, "Open Gmail web")
			gConn, err := openGmailWeb(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to open Gmail website")
			}
			if appName == YoutubeApp && resources.LFixtVal != nil {
				// For Youtube App, the current lacros with Gmail is the only lacros window.
				// Leave a new tab to keep the browser alive for further testing.
				defer func() {
					gConn.Close()
					if err := cuj.KeepNewTab(ctx, bTconn); err != nil {
						testing.ContextLog(ctx, "Failed to keep new tab: ", err)
					}
				}()
			} else {
				defer gConn.Close()
				defer gConn.CloseTarget(cleanupResourceCtx)
			}

			ytApp, ok := videoApp.(*YtApp)
			// Only do PiP testing for YT APP and when logged in as premium user.
			if ok && ytApp.isPremiumAccount() {
				if err = ytApp.checkYoutubeAppPIP(ctx); err != nil {
					return errors.Wrap(err, "youtube app smaller video preview window is not shown")
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

			// Pause and resume video playback.
			if err := videoApp.PauseAndPlayVideo(ctx); err != nil {
				return errors.Wrap(err, "failed to pause and play video")
			}

			if extendedDisplay {
				if err := moveGmailWindow(ctx, tconn, resources); err != nil {
					return errors.Wrap(err, "failed to move Gmail window between main display and extended display")
				}
				if appName == YoutubeWeb {
					if err := moveYTWebWindow(ctx, tconn, resources); err != nil {
						return errors.Wrap(err, "failed to move YT Web window to internal display")
					}
				}
			}

			// Before recording the metrics, check if there is any tab crashed.
			if err := tabChecker.Check(ctx); err != nil {
				return errors.Wrap(err, "tab renderer crashed")
			}
			return nil
		}); err != nil {
			return errors.Wrapf(err, "failed to run %q video playback", appName)
		}
	}

	pv := perf.NewValues()

	// We'll collect Browser.StartTime for both YouTube-Web and YouTube-App
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
		}, float64(appStartTime.Milliseconds()))
	}

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
		return errors.Wrap(err, "failed to record the performance metrics")
	}
	if err := pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save performance metrics")
	}
	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}
	return nil
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
		return -1, errors.Wrap(err, "failed to get all windows")
	}
	if len(all) != 1 {
		return -1, errors.Errorf("expect 1 window, got %d", len(all))
	}
	return all[0].ID, nil
}

// moveGmailWindow switches Gmail to the extended display and switches back to internal display.
func moveGmailWindow(ctx context.Context, tconn *chrome.TestConn, testRes TestResources) error {
	return uiauto.Combine("switch to gmail and move it between two displays",
		testRes.UIHandler.SwitchWindow(),
		cuj.SwitchWindowToDisplay(ctx, tconn, testRes.Kb, true),  // Move to external display.
		uiauto.Sleep(2*time.Second),                              // Keep the window in external display for 2 second.
		cuj.SwitchWindowToDisplay(ctx, tconn, testRes.Kb, false), // Move to internal display.
	)(ctx)
}

// moveYTWebWindow switches Youtube Web to the internal display.
func moveYTWebWindow(ctx context.Context, tconn *chrome.TestConn, testRes TestResources) error {
	return uiauto.Combine("switch to YT Web and move it to internal display",
		testRes.UIHandler.SwitchWindow(),
		cuj.SwitchWindowToDisplay(ctx, tconn, testRes.Kb, false), // Move to internal display.
	)(ctx)
}

func installYoutubeApp(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	cleanupCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	device, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up ARC device")
	}
	defer device.Close(cleanupCtx)

	installErr := playstore.InstallOrUpdateApp(ctx, a, device, youtubePkg, &playstore.Options{TryLimit: -1})

	if err := apps.Close(cleanupCtx, tconn, apps.PlayStore.ID); err != nil {
		// Leaving PlayStore open will impact the logic of detecting fullscreen
		// mode in this test case. We fail the test if this happens.
		return errors.Wrap(err, "failed to close Play Store")
	}

	return installErr
}
