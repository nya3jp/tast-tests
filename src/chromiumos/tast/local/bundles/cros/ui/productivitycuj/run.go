// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/cuj/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/input/voice"
	"chromiumos/tast/testing"
)

// Run runs the specified user scenario in productivity with different CUJ tiers.
func Run(ctx context.Context, cr *chrome.Chrome, app ProductivityApp, tier cuj.Tier, tabletMode bool, outDir, expectedText, testFileLocation string) (err error) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	defer tconn.Close()

	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err := cuj.GetBrowserStartTime(ctx, cr, tconn, tabletMode)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

	// Shorten the context to resume battery charging.
	cleanUpBatteryCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(cleanUpBatteryCtx)

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	testing.ContextLog(ctx, "Start recording actions")
	recorder, err := cuj.NewRecorder(ctx, cr, cuj.MetricConfigs()...)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return err != nil }, cr, "ui_dump")

	productivityTimeout := 80 * time.Second
	if tier == cuj.Premium {
		productivityTimeout = 120 * time.Second
	}
	sheetName, err := app.CreateSpreadsheet(ctx)
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
		if err := app.End(ctx); err != nil {
			return err
		}

		// Wait for productivityTimeout expires in goroutine and get GPU result.
		if err := <-errc; err != nil {
			return errors.Wrap(err, "failed to collect GPU counters")
		}
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to conduct the recorder task")
	}

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
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

	// Since the DUT may be connected to multiple audio devices, the "Loopback Playback/Capture" option will not be displayed directly in the detailed page of the audio settings
	// (you need to scroll down the page to find it). Therefore, disable Bluetooth to display options before enabling Aloop. Otherwise, it will fail in voice.EnableAloop().
	isBtEnabled, err := bluetooth.IsEnabled(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get bluetooth status")
	}

	if isBtEnabled {
		testing.ContextLog(ctx, "Start to disable bluetooth")
		if err := bluetooth.Disable(ctx); err != nil {
			return errors.Wrap(err, "failed to disable bluetooth")
		}
		defer bluetooth.Enable(ctx)
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
