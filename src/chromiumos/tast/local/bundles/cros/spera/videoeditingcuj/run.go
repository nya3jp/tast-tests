// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videoeditingcuj contains the test code for VideoEditingOnTheWeb CUJ.
package videoeditingcuj

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/spera/videoeditingcuj/wevideo"
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

const (
	weVideoTitle    = "WeVideo"
	googleDocsTitle = "Google Docs"
	clip1           = "Springtime Birds Migration Northern Parula Warbler 4K"
	clip2           = "Hammock and Beach Chairs"
	clipTime1       = "00:23:00"
	clipTime2       = "00:04:00"
	videoTrack      = "Video 1"
	textTrack       = "Text 1"
	demoText        = "Springtime Birds"
	docParagraph    = "The Little Prince's story follows a young prince who visits various planets in space, including Earth, and addresses themes of loneliness, friendship, love, and loss."
)

// Run runs the EDUVideoEditingCUJ test.
func Run(ctx context.Context, outDir, traceConfigPath string, cr *chrome.Chrome, tabletMode bool, bt browser.Type) error {
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

	uiHdl, err := uiHandler(ctx, tconn, tabletMode)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, bt)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	br := cr.Browser()
	var bTconn *chrome.TestConn
	if l != nil {
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get lacros test API conn")
		}
		br = l.Browser()
	}
	defer cuj.CloseAllWindows(ctx, tconn)

	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanupCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}
	if traceConfigPath != "" {
		recorder.EnableTracing(outDir, traceConfigPath)
	}
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		return videoEditingScenario(ctx, tconn, cr, kb, uiHdl, tabletMode, outDir, br)
	}); err != nil {
		return errors.Wrap(err, "failed to run the video editing on the WeVideo web")
	}

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
		return errors.Wrap(err, "failed to record")
	}
	if err = pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to store values")
	}
	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}
	return nil
}

func videoEditingScenario(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, kb *input.KeyboardEventWriter,
	uiHdl cuj.UIActionHandler, tabletMode bool, outDir string, br *browser.Browser) error {
	hasError := true
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()
	account := cr.Creds().User
	w := wevideo.NewWeVideo(tconn, kb, uiHdl, tabletMode, br)
	if err := w.Open()(ctx); err != nil {
		return errors.Wrap(err, "failed to open the WeVideo page")
	}
	defer cleanup(cleanupCtx, tconn, cr, w, outDir, func() bool { return hasError })

	if err := uiauto.Combine("run the video editing scenario",
		maximizeBrowserWindow(ctx, tconn, tabletMode),
		w.Login(account),
		w.Create(),
		w.AddStockVideo(clip1, "", clipTime1, videoTrack),
		w.AddStockVideo(clip2, clip1, clipTime2, videoTrack),
		w.AddText(clip1, textTrack, demoText),
	)(ctx); err != nil {
		return err
	}

	if err := googleapps.NewGoogleDocs(ctx, tconn, br, uiHdl, true); err != nil {
		return err
	}
	defer docCleanup(cleanupCtx, tconn, cr, uiHdl, outDir, func() bool { return hasError })

	if err := uiauto.Combine("run the video editing scenario",
		googleapps.EditDoc(tconn, kb, docParagraph),
		uiHdl.SwitchToAppWindowByName("Chrome", weVideoTitle),
		w.AddTransition(clip2),
		w.PlayVideo(clip1),
	)(ctx); err != nil {
		return err
	}
	hasError = false
	return nil
}

// maximizeBrowserWindow returns an action that maximize the WeVideo window to show all the browser UI elements for precise clicking.
func maximizeBrowserWindow(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) action.Action {
	return func(ctx context.Context) error {
		if tabletMode {
			return nil
		}
		// Find the WeVideo browser window.
		window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
			return (w.WindowType == ash.WindowTypeBrowser || w.WindowType == ash.WindowTypeLacros) && strings.Contains(w.Title, weVideoTitle)
		})
		if err != nil {
			return errors.Wrap(err, "failed to find the WeVideo window")
		}
		if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateMaximized); err != nil {
			// Just log the error and try to continue.
			testing.ContextLog(ctx, "Try to continue the test even though maximizing the WeVideo window failed: ", err)
		}
		return nil
	}
}

func uiHandler(ctx context.Context, tconn *chrome.TestConn, tabletMode bool) (cuj.UIActionHandler, error) {
	var uiHdl cuj.UIActionHandler
	var err error
	if tabletMode {
		if uiHdl, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return uiHdl, errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if uiHdl, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return uiHdl, errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	return uiHdl, nil
}

func cleanup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, w *wevideo.WeVideo, outDir string, hasError func() bool) {
	// The screenshot and ui tree dump must been taken before the connection is closed.
	faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, hasError, cr, "ui_dump")
	w.Close(ctx)
}

func docCleanup(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, uiHdl cuj.UIActionHandler, outDir string, hasError func() bool) {
	// docCleanup switches to the document page and deletes it.
	docCleanup := uiauto.Combine("switch to the document page and delete it",
		uiHdl.SwitchToAppWindowByName("Chrome", googleDocsTitle),
		googleapps.DeleteDoc(tconn),
	)
	// If case fails, dump the last screen before deleting the document.
	faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, hasError, cr, "ui_dump_last")
	if err := docCleanup(ctx); err != nil {
		// Only log the error.
		testing.ContextLog(ctx, "Failed to clean up the document: ", err)
	}
}
