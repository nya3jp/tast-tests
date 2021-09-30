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
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

// Run runs the specified user scenario in productivity with different CUJ tiers.
func Run(ctx context.Context, cr *chrome.Chrome, app ProductivityApp, tier cuj.Tier, tabletMode bool, outDir, testFileLocation string) error {
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

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return true }, cr, "ui_dump")

	productivityTimeout := 90 * time.Second
	// Since the execution time of the premium tier is longer than the execution time of the basic and plus tiers, the timeout is slightly extended.
	if tier == cuj.Premium {
		productivityTimeout = 130 * time.Second
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
