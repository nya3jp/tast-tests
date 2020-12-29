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
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power/setup"
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
	Cr    *chrome.Chrome
	Tconn *chrome.TestConn
	A     *arc.ARC
	Kb    *input.KeyboardEventWriter
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
	url string
	// quality is the string that test will look for in youtube
	// "Settings / Quality" menu to change video playback quality.
	quality string
}

var basicVideoSrc = []videoSrc{
	{"https://www.youtube.com/watch?v=suWsd372pQE", "1080p"},
	{"https://www.youtube.com/watch?v=b3wcQqINmE4", "720p60"},
	{"https://www.youtube.com/watch?v=b3wcQqINmE4", "1080p60"},
}

var plusVideoSrc = []videoSrc{
	{"https://www.youtube.com/watch?v=b3wcQqINmE4", "2160p60"},
}

// Run runs the VideoCUJ test.
func Run(ctx context.Context, resources TestResources, param TestParams) (retErr error) {
	var (
		cr              = resources.Cr
		tconn           = resources.Tconn
		a               = resources.A
		kb              = resources.Kb
		outDir          = param.OutDir
		appName         = param.App
		tabletMode      = param.TabletMode
		tier            = param.Tier
		extendedDisplay = param.ExtendedDisplay
	)

	testing.ContextLogf(ctx, "Run app appName: %s tabletMode: %t, extendedDisplay: %t", appName, tabletMode, extendedDisplay)

	tabChecker, err := cuj.NewTabCrashChecker(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create TabCrashChecker")
	}

	var uiHandler cuj.UIActionHandler
	if tabletMode {
		if uiHandler, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if uiHandler, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	defer uiHandler.Close()

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

	// Give 5 seconds to resume battery charging.
	cleanupDischargeCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	setBatteryNormal, err := setup.SetBatteryDischarge(ctx, 50)
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

	for _, videoSource := range videoSources {
		// Repeat the run for different video source.
		if err = recorder.Run(ctx, func(ctx context.Context) error {
			var videoApp VideoApp
			switch appName {
			case YoutubeWeb:
				videoApp = NewYtWeb(cr, tconn, kb, videoSource, extendedDisplay, ui, uiHandler)
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

			if err := uiHandler.SwitchWindow()(ctx); err != nil {
				return errors.Wrap(err, "failed to switch back to video playing")
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
			return errors.Wrap(err, "failed to run recorder")
		}
	}

	pv := perf.NewValues()

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
		Multiple:  true,
	}, float64(browserStartTime.Milliseconds()))

	if err := recorder.Record(ctx, pv); err != nil {
		return errors.Wrap(err, "failed to record result")
	}
	if err := pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed saving perf data")
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
