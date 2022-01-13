// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package imageeditingcuj contains imageedit CUJ test cases library.
package imageeditingcuj

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// Run implements the main logic of the EDUImageEditingCUJ test case.
func Run(ctx context.Context, cr *chrome.Chrome, googlePhotos *GooglePhotos, tabletMode bool, outDir, testImage, testImageLocation string) (retErr error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	_, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, browser.TypeAsh)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

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

	// Shorten the context to clean up the files created in the test case.
	cleanUpResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	fileDestination := filepath.Join(filesapp.DownloadPath, testImage)
	if err := fsutil.CopyFile(testImageLocation, fileDestination); err != nil {
		return errors.Wrapf(err, "failed to copy %s to Downloads", testImage)
	}
	defer func(ctx context.Context) {
		faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_dump")
		googlePhotos.CleanUp()
		googlePhotos.Close(ctx)
		os.Remove(fileDestination)
	}(cleanUpResourceCtx)

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Start recording actions")
	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, options, cuj.MetricConfigs([]*chrome.TestConn{tconn})...)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		return uiauto.Combine("main logic of image editing cuj",
			googlePhotos.Open(),
			googlePhotos.Upload(testImage),
			googlePhotos.AddFilters(),
			googlePhotos.Edit(),
			googlePhotos.Rotate(),
			googlePhotos.ReduceColor(),
			googlePhotos.Crop(),
			googlePhotos.UndoEdit(),
		)(ctx)
	}); err != nil {
		return errors.Wrap(err, "failed to conduct the recorder task")
	}

	if err := recorder.Record(ctx, pv); err != nil {
		return errors.Wrap(err, "failed to record the data")
	}

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	if err := pv.Save(outDir); err != nil {
		return errors.Wrap(err, "failed to save perf data")
	}

	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}

	return nil
}
