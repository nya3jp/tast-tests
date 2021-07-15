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

// videoSrc struct defines video src for testing.
type videoSrc struct {
	url   string
	title string
	// quality is the string that test will look for in youtube
	// "Settings / Quality" menu to change video playback quality.
	quality string
}

var basicVideoSrc = []videoSrc{
	{
		"https://www.youtube.com/watch?v=suWsd372pQE",
		`"Go Beyond" 4K Ultra HD Time Lapse`,
		"1080p",
	},
	{
		"https://www.youtube.com/watch?v=b3wcQqINmE4",
		"8K Videos | Collection of World's nature  UltraHD (120 FPS)",
		"720p60",
	},
	{
		"https://www.youtube.com/watch?v=b3wcQqINmE4",
		"8K Videos | Collection of World's nature  UltraHD (120 FPS)",
		"1080p60",
	},
}

var plusVideoSrc = []videoSrc{
	{
		"https://www.youtube.com/watch?v=b3wcQqINmE4",
		"8K Videos | Collection of World's nature  UltraHD (120 FPS)",
		"2160p60",
	},
}

// Run runs the VideoCUJ test.
func Run(ctx context.Context, resources TestResources, param TestParams) (retErr error) {
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

	// Give 5 seconds to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	recorder, err := cuj.NewRecorder(ctx, tconn, cuj.MetricConfigs()...)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanupRecorderCtx)

	// Give 5 seconds to resume battery charging. It is critical to ensure
	// setBatteryNormal can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupDischargeCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(cleanupDischargeCtx)

	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

	videoSources := basicVideoSrc
	if tier == cuj.Plus {
		videoSources = plusVideoSrc
	}

	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create new ARC device")
	}
	defer d.Close(cleanupCtx)

	hasError := func() bool { return retErr != nil }
	defer faillog.DumpUITreeOnError(cleanupCtx, outDir, hasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, outDir, hasError)

	openGmailWeb := func(ctx context.Context) (*chrome.Conn, error) {
		conn, err := uiHandler.NewChromeTab(ctx, cr, "https://mail.google.com", true)
		if err != nil {
			return conn, errors.Wrap(err, "failed to open gmail web page")
		}
		if err := webutil.WaitForQuiescence(ctx, conn, 2*time.Minute); err != nil {
			return conn, errors.Wrap(err, "failed to wait for gmail page to finish loading")
		}
		// YouTube sometimes pops up a prompt to notice users how to operate YouTube
		// if there're new features. Dismiss prompt if it exist.
		gotItPrompt := nodewith.Name("Got it").Role(role.Button)
		ui.IfSuccessThen(
			ui.WaitUntilExists(gotItPrompt),
			uiHandler.ClickUntil(
				gotItPrompt,
				ui.WithTimeout(2*time.Second).WaitUntilGone(gotItPrompt),
			),
		)
		return conn, nil
	}

	for _, videoSource := range videoSources {
		// Repeat the run for different video source.
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			var videoApp VideoApp
			switch appName {
			case YoutubeWeb:
				videoApp = NewYtWeb(cr, tconn, kb, videoSource, extendedDisplay, ui, uiHandler)
			case YoutubeApp:
				videoApp = NewYtApp(tconn, kb, a, d, videoSource)
			}

			if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
				return errors.Wrapf(err, "failed to open %s", appName)
			}
			defer videoApp.Close(cleanupCtx)

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
			defer gConn.Close()
			defer gConn.CloseTarget(cleanupCtx)

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
				if err := moveGmailWindow(ctx, gConn, ui, resources); err != nil {
					return errors.Wrap(err, "failed to move Gmail window between main display and extended display")
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

	if err := recorder.Record(ctx, pv); err != nil {
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
func moveGmailWindow(ctx context.Context, gConn *chrome.Conn, ui *uiauto.Context, testRes TestResources) error {
	internalDisplay := nodewith.ClassName("RootWindow-0").Role(role.Window)
	externalDisplay := nodewith.ClassName("RootWindow-1").Role(role.Window)
	gmailWeb := nodewith.Name("Compose an email").Role(role.Button)

	return uiauto.Combine("focus gmail and move it between two displays",
		testRes.UIHandler.SwitchWindow(),
		testRes.Kb.AccelAction("Search+Alt+M"),
		ui.WithTimeout(3*time.Second).WaitUntilExists(gmailWeb.Ancestor(externalDisplay)),
		testRes.Kb.AccelAction("Search+Alt+M"),
		ui.WithTimeout(3*time.Second).WaitUntilExists(gmailWeb.Ancestor(internalDisplay)),
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

	installErr := playstore.InstallApp(ctx, a, device, youtubePkg, -1)

	if err := apps.Close(cleanupCtx, tconn, apps.PlayStore.ID); err != nil {
		// Leaving PlayStore open will impact the logic of detecting fullscreen
		// mode in this test case. We fail the test if this happens.
		return errors.Wrap(err, "failed to close Play Store")
	}

	return installErr
}
