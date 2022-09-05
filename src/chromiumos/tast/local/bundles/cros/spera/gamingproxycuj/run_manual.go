// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gamingproxycuj contains the test code for GamingProxyCUJ.
package gamingproxycuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/googleapps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// RunＭanual runs the Gaming Proxy Manual CUJ test.
func RunＭanual(ctx context.Context, cr *chrome.Chrome, outDir string, tabletMode bool, bt browser.Type,
	videoOption VideoOption, manualTestTime time.Duration) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kb.Close()

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	testing.ContextLog(ctx, "Start to get browser start time")
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, bt)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	br := cr.Browser()
	var bTconn *chrome.TestConn
	if l != nil {
		defer l.Close(ctx)
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get lacros test API conn")
		}
		br = l.Browser()
	}

	// uiHandler will be assigned with different instances for clamshell and tablet mode.
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

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, options)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}

	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "could not find the Chrome app")
	}
	if err := googleapps.NewGoogleDocs(ctx, tconn, br, uiHandler, true); err != nil {
		return err
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
		// Maximize the Google Docs window to delete Docs.
		if err := cuj.MaximizeBrowserWindow(ctx, tconn, tabletMode, googleDocsTitle); err != nil {
			testing.ContextLog(ctx, "Failed to maximize the Google Docs page")
		}
		if err := googleapps.DeleteDoc(tconn)(ctx); err != nil {
			// Only log the error.
			testing.ContextLog(ctx, "Failed to clean up the document: ", err)
		}
		cuj.CloseAllWindows(ctx, tconn)
	}(cleanupCtx)

	video, err := NewCrosVideo(ctx, tconn, uiHandler, br)
	if err != nil {
		return errors.Wrap(err, "failed to open cros video")
	}
	defer video.Close(cleanupCtx)

	// Maximize all windows to ensure a consistent state.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
	}); err != nil {
		return errors.Wrap(err, "failed to maximize windows")
	}

	if err := uiauto.Combine("manual testing",
		video.Play(videoOption),
		uiHandler.SwitchToAppWindowByName(chromeApp.Name, googleDocsTitle),
		putDocsWindowSideBySide(tabletMode, tconn, bTconn),
		uiauto.NamedCombine("start manual testing", sendNotification(tconn, "Start manual testing")),
	)(ctx); err != nil {
		return err
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		return uiauto.Sleep(manualTestTime)(ctx)
	}); err != nil {
		return errors.Wrap(err, "failed to run the gaming proxy manual manually cuj scenario")
	}

	if err := uiauto.Combine("stop manual testing",
		uiauto.NamedAction("stop manual testing", sendNotification(tconn, "Stop manual testing")),
		uiauto.IfFailThen(video.Exists(), uiHandler.SwitchToAppWindowByName(chromeApp.Name, crosVideoTitle)),
		video.Pause(),
	)(ctx); err != nil {
		return err
	}
	decodedFrames, droppedFrames, droppedFramesPer, err := video.FramesData(ctx)
	if err != nil {
		return err
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))
	pv.Set(perf.Metric{
		Name:      "TPS.AppSpot.DecodedFrames",
		Unit:      "count",
		Direction: perf.BiggerIsBetter,
	}, decodedFrames)
	pv.Set(perf.Metric{
		Name:      "TPS.AppSpot.DroppedFrames",
		Unit:      "count",
		Direction: perf.SmallerIsBetter,
	}, droppedFrames)
	pv.Set(perf.Metric{
		Name:      "TPS.AppSpot.DroppedFramesPer",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, droppedFramesPer)

	if err := recorder.Record(ctx, pv); err != nil {
		return errors.Wrap(err, "failed to report")
	}
	if err = pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store values")
	}
	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}

	return nil
}

// sendNotification returns an action that creates the notification to remind the tester to the next action.
func sendNotification(tconn *chrome.TestConn, message string) action.Action {
	return func(ctx context.Context) error {
		const (
			uiTimeout = 30 * time.Second
			title     = "[Manual Testing]"
		)
		notificationType := browser.NotificationTypeBasic
		_, err := browser.CreateTestNotification(ctx, tconn, notificationType, title, message)
		if err != nil {
			return errors.Wrapf(err, "failed to create %s notification: ", notificationType)
		}
		// Wait for the last notification to finish creating.
		if _, err := ash.WaitForNotification(ctx, tconn, uiTimeout, ash.WaitTitle(title)); err != nil {
			return errors.Wrap(err, "failed to wait for notification")
		}
		return nil
	}
}
