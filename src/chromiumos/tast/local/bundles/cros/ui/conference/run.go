// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
)

// Cleanup releases the resources which the case used.
type Cleanup func(context.Context) error

// Prepare prepares conference room link before testing.
type Prepare func(context.Context) (string, Cleanup, error)

// Run runs the specified user scenario in conference room with different CUJ tiers.
func Run(ctx context.Context, cr *chrome.Chrome, conf Conference, prepare Prepare, tier, outDir string, tabletMode, extendedDisplay bool) error {
	// Shorten context a bit to allow for cleanup.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	inviteLink, cleanup, err := prepare(ctx)
	if err != nil {
		return err
	}
	defer cleanup(cleanUpCtx)

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

	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(cleanUpCtx)

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

	meetTimeout := 80 * time.Second
	if tier == "plus" {
		meetTimeout = 160 * time.Second
	}
	if tier == "premium" {
		meetTimeout = 180 * time.Second
	}
	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		// Collect GPU metrics in goroutine while other tests are being executed.
		errc := make(chan error, 1) // Buffered channel to make sure goroutine will not be blocked.
		gpuCtx, cancel := context.WithTimeout(ctx, meetTimeout+5*time.Second)
		defer cancel() // Make sure goroutine ctx will be cancelled before return.
		go func() {
			errc <- graphics.MeasureGPUCounters(gpuCtx, meetTimeout, pv)
		}()

		if err := conf.Join(ctx, inviteLink); err != nil {
			return err
		}

		// Basic
		if err := conf.SwitchTabs(ctx); err != nil {
			return err
		}

		if err := conf.VideoAudioControl(ctx); err != nil {
			return err
		}

		if err := conf.ChangeLayout(ctx); err != nil {
			return err
		}

		// Plus and premium tier.
		if tier == "plus" || tier == "premium" {
			if extendedDisplay {
				if err := conf.ExtendedDisplayPresenting(ctx); err != nil {
					return err
				}
			} else {
				if err := conf.PresentSlide(ctx); err != nil {
					return err
				}
				if err := conf.StopPresenting(ctx); err != nil {
					return err
				}
			}
		}

		// Premium tier.
		if tier == "premium" {
			if err := conf.BackgroundBlurring(ctx); err != nil {
				return err
			}
		}

		// Wait for meetTimeout expires in goroutine and get GPU result.
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
