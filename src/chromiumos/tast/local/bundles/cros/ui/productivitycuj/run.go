// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package productivitycuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// Run runs the specified user scenario in productivity with different CUJ tiers.
func Run(ctx context.Context, cr *chrome.Chrome, app ProductivityApp, tier cuj.Tier, tabletMode bool, bt browser.Type, outDir, traceConfigPath, sampleSheetURL, expectedText, testFileLocation string) (err error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}

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
	app.SetBrowser(br)

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

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	testing.ContextLog(ctx, "Start recording actions")
	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		return errors.Wrap(err, "failed to add metrics to recorder")
	}
	defer browser.CloseAllTabs(ctx, tconn)
	if traceConfigPath != "" {
		recorder.EnableTracing(outDir, traceConfigPath)
	}
	// Shorten the context to clean up the files created in the test case.
	cleanUpResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer app.Cleanup(cleanUpResourceCtx, sheetName)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return err != nil }, cr, "ui_dump")

	productivityTimeout := 90 * time.Second
	// Since the execution time of the premium tier is longer than the execution time of the basic and plus tiers, the timeout is slightly extended.
	if tier == cuj.Premium {
		productivityTimeout = 130 * time.Second
	}
	sheetName, err := app.CreateSpreadsheet(ctx, cr, sampleSheetURL, outDir)
	if err != nil {
		return err
	}
	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Collect GPU metrics in goroutine while other tests are being executed.

		errc := make(chan error, 1) // Buffered channel to make sure goroutine will not be blocked.
		go func() {
			errc <- graphics.MeasureGPUCounters(ctx, productivityTimeout, pv)
		}()

		err := app.CreateDocument(ctx)
		if err != nil {
			return err
		}
		if err := app.CreateSlides(ctx); err != nil {
			return err
		}
		if err := app.OpenSpreadsheet(ctx, sheetName); err != nil {
			return err
		}
		if err := app.MoveDataFromDocToSheet(ctx); err != nil {
			return err
		}
		if err := app.MoveDataFromSheetToDoc(ctx); err != nil {
			return err
		}
		if err := app.ScrollPage(ctx); err != nil {
			return err
		}
		if err := app.SwitchToOfflineMode(ctx); err != nil {
			return err
		}
		if err := app.UpdateCells(ctx); err != nil {
			return err
		}
		// For the "Premium" tier, it will have another process that uses voice-to-text (VTT) to enter text directly into the document.
		if tier == cuj.Premium {
			if err := voiceToTextTesting(ctx, app, tconn, expectedText, testFileLocation); err != nil {
				return errors.Wrap(err, "failed to test voice to text")
			}
		}

		// Wait for productivityTimeout expires in goroutine and get GPU result.
		if err := <-errc; err != nil {
			return errors.Wrap(err, "failed to collect GPU counters")
		}
		return nil
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

func voiceToTextTesting(ctx context.Context, app ProductivityApp, tconn *chrome.TestConn, expectedText, testFileLocation string) error {

	playAudio := func(ctx context.Context) error {
		// Set up the test audio file.
		audioInput := audio.TestRawData{
			Path:          testFileLocation,
			BitsPerSample: 16,
			Channels:      2,
			Rate:          48000,
		}

		// Playback function by CRAS.
		playCmd := crastestclient.PlaybackFileCommand(
			ctx, audioInput.Path,
			audioInput.Duration,
			audioInput.Channels,
			audioInput.Rate)
		playCmd.Start()

		return playCmd.Wait()
	}

	// Reserve time to remove input file and unload ALSA loopback at the end of the test.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cleanup, err := voice.EnableAloop(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to load ALSA loopback module")
	}
	defer cleanup(cleanupCtx)

	if err := app.VoiceToTextTesting(ctx, expectedText, playAudio); err != nil {
		return err
	}

	return nil
}
