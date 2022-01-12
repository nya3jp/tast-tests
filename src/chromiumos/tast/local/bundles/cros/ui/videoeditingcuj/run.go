// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package videoeditingcuj contains the test code for VideoEditingOnTheWeb CUJ.
package videoeditingcuj

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/googleapps"
	"chromiumos/tast/local/bundles/cros/ui/videoeditingcuj/wevideo"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
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
	docParagraph    = "The Little Prince's story follows a young prince who visits various planets in space, " +
		"including Earth, and addresses themes of loneliness, friendship, love, and loss."
)

// Run runs the EDUVideoEditingCUJ test.
func Run(ctx context.Context, outDir string, cr *chrome.Chrome, tabletMode bool) error {
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

	testing.ContextLog(ctx, "Start to get browser start time")
	_, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, browser.TypeAsh)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, options, cuj.MetricConfigs([]*chrome.TestConn{tconn})...)
	if err != nil {
		return errors.Wrap(err, "failed to create a recorder")
	}
	defer recorder.Close(cleanupCtx)

	if err := recorder.Run(ctx, func(ctx context.Context) (retErr error) {
		account := cr.Creds().User
		w := wevideo.NewWeVideo(tconn, kb, uiHandler, tabletMode)
		if err := w.Open(cr)(ctx); err != nil {
			return errors.Wrap(err, "failed to open the WeVideo page")
		}
		defer func(ctx context.Context) {
			// The screenshot and ui tree dump must been taken before the connection is closed.
			faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
			w.Close(ctx)
			cuj.CloseAllWindows(ctx, tconn)
		}(cleanupCtx)

		// Maximize the WeVideo window to show all the browser UI elements for precise clicking.
		if !tabletMode {
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
		}
		if err := uiauto.Combine("run the video editing scenario",
			w.Login(account),
			w.Create(),
			w.AddStockVideo(clip1, clipTime1, videoTrack),
			w.AddStockVideo(clip2, clipTime2, videoTrack),
			w.AddText(clip1, textTrack, demoText),
		)(ctx); err != nil {
			return err
		}
		// docCleanup switches to the document page and deletes it.
		docCleanup := uiauto.Combine("switch to the document page and delete it",
			uiHandler.SwitchToAppWindowByName("Chrome", googleDocsTitle),
			googleapps.DeleteDoc(tconn),
		)
		if err := googleapps.NewGoogleDocs(cr, tconn, true)(ctx); err != nil {
			return err
		}
		defer func(ctx context.Context) {
			// If case fails, dump the last screen before deleting the document.
			faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump_last")
			if err := docCleanup(ctx); err != nil {
				// Only log the error.
				testing.ContextLog(ctx, "Failed to clean up the document: ", err)
			}
		}(cleanupCtx)

		if err := uiauto.Combine("run the video editing scenario",
			googleapps.EditDoc(tconn, kb, docParagraph),
			uiHandler.SwitchToAppWindowByName("Chrome", weVideoTitle),
			w.AddTransition(clip2),
			w.PlayVideo(clip1),
		)(ctx); err != nil {
			return err
		}
		return nil
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
