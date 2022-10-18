// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gamingproxycuj contains the test code for GamingProxyCUJ.
package gamingproxycuj

import (
	"context"
	"fmt"
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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/googleapps"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

const (
	crosVideoTitle  = "CrosVideo"
	googleDocsTitle = "Google Docs"
)

// Run runs the GamingProxyCUJ test.
func Run(ctx context.Context, cr *chrome.Chrome, outDir, traceConfigPath string, tabletMode bool, bt browser.Type, videoOption VideoOption) (retErr error) {
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
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}
	if traceConfigPath != "" {
		recorder.EnableTracing(outDir, traceConfigPath)
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
		if err := maximizeWindowSize(ctx, tabletMode, tconn, bTconn); err != nil {
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

	if err := uiauto.NamedCombine("initial scenario",
		video.Play(videoOption),
		uiHandler.SwitchToAppWindowByName(chromeApp.Name, googleDocsTitle),
		putDocsWindowSideBySide(tabletMode, tconn, bTconn),
	)(ctx); err != nil {
		return err
	}

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		return gamingProxyScenario(ctx, tconn, kb, video)
	}); err != nil {
		return errors.Wrap(err, "failed to run the gaming proxy scenario")
	}

	if err := uiauto.Combine("pause video",
		uiHandler.SwitchToAppWindowByName(chromeApp.Name, crosVideoTitle),
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
		Name:      "TPS.CrosVideo.DroppedFramesPct",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, droppedFramesPer)

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
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

func gamingProxyScenario(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, video *CrosVideo) error {
	const (
		docParagraph  = "The Little Prince's story follows a young prince who visits various planets in space."
		repeatTimeout = 15 * time.Minute
		retryTimes    = 3
	)
	taskNumber := 0
	repeatTask := func(ctx context.Context) error {
		var color, fontSize string
		if taskNumber%2 == 0 {
			color = "red"
			fontSize = "10"
		} else {
			color = "blue"
			fontSize = "8"
		}
		ui := uiauto.New(tconn)
		reloadDialog := nodewith.Name("Unable to load file").Role(role.Dialog)
		reloadButton := nodewith.Name("Reload").Role(role.Button).Ancestor(reloadDialog)

		// Some low-end DUTs sometimes click on the node and don't respond, or nodes can't be found.
		// Add retry to solve this problem.
		return uiauto.Retry(retryTimes, uiauto.NamedCombine(fmt.Sprintf("repeat task, number %d", taskNumber),
			uiauto.IfSuccessThen(ui.Exists(reloadButton), ui.LeftClick(reloadButton)),
			googleapps.EditDoc(tconn, kb, docParagraph),
			kb.AccelAction("Ctrl+A"),
			googleapps.ChangeDocTextColor(tconn, color),
			googleapps.ChangeDocFontSize(tconn, fontSize),
			googleapps.UndoDoc(tconn),
			googleapps.RedoDoc(tconn),
			kb.AccelAction("Backspace"),
			video.VerifyPlaying,
		))(ctx)
	}
	// Repeat the task for 15 minutes.
	now := time.Now()
	after := now.Add(repeatTimeout)
	for {
		taskNumber++
		if err := repeatTask(ctx); err != nil {
			return err
		}
		now = time.Now()
		if now.After(after) {
			break
		}
	}

	return nil
}

func putDocsWindowSideBySide(tabletMode bool, tconn, bTconn *chrome.TestConn) action.Action {
	return func(ctx context.Context) error {
		// Google Docs requires at least 320 px height to edit files correctly.
		const expectedHeight = 320
		return setWindowSizeWithWorkArea(ctx, tabletMode, tconn, bTconn, expectedHeight, 0)
	}
}

// maximizeWindowSize sets the last focused window to maximized size.
func maximizeWindowSize(ctx context.Context, tabletMode bool, tconn, bTconn *chrome.TestConn) error {
	return setWindowSizeWithWorkArea(ctx, tabletMode, tconn, bTconn, 0, 0)
}

// setWindowSizeWithWorkArea sets the last focused window with WorkArea size.
// The default values ​​are the height and width of the workArea for the internal display.
// For lacros windows, use the lacros TestConn. For ash, use the ash TestConn.
func setWindowSizeWithWorkArea(ctx context.Context, tabletMode bool, tconn, bTconn *chrome.TestConn, expectedHeight, expectedWidth int) error {
	// Tablet mode can't set the window size, so no need to set it.
	if tabletMode {
		return nil
	}
	// Obtain the latest display info after rotating the display.
	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain internal display info")
	}
	if expectedHeight == 0 {
		expectedHeight = info.WorkArea.Height
	}
	if expectedWidth == 0 {
		expectedWidth = info.WorkArea.Width
	}
	if bTconn == nil {
		bTconn = tconn
	}
	return setWindowSize(ctx, bTconn, expectedHeight, expectedWidth)
}

// setWindowSize sets the last focused window to specific size.
// For lacros windows, use the lacros TestConn. For ash, use the ash TestConn.
func setWindowSize(ctx context.Context, tconn *chrome.TestConn, height, width int) error {
	script := fmt.Sprintf(`async () => {
        const win = await tast.promisify(chrome.windows.getLastFocused)();
        await tast.promisify(chrome.windows.update)(win.id, {width: %d, height: %d, state:"normal"});
	}`, width, height)

	if err := tconn.Call(ctx, nil, script); err != nil {
		return errors.Wrap(err, "setting window size failed")
	}

	return nil
}
