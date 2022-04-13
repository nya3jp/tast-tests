// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package conference

import (
	"context"
	"path/filepath"
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

// Cleanup releases the resources which the case used.
type Cleanup func(context.Context) error

// Prepare prepares conference room link before testing.
type Prepare func(context.Context) (string, Cleanup, error)

// Run runs the specified user scenario in conference room with different CUJ tiers.
func Run(ctx context.Context, cr *chrome.Chrome, conf Conference, prepare Prepare, tier, outDir string, tabletMode bool, roomSize int) (retErr error) {
	// Shorten context a bit to allow for cleanup.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	inviteLink, cleanup, err := prepare(ctx)
	if err != nil {
		return err
	}
	defer cleanup(cleanUpCtx)
	// Dump the UI tree to the service/faillog subdirectory.
	// Don't dump directly into outDir
	// because it might be overridden by the test faillog after pulled back to remote server.
	defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, filepath.Join(outDir, "service"), func() bool { return retErr != nil }, cr, "ui_dump")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	_, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, false)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

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

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	testing.ContextLog(ctx, "Start recording actions")
	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.MetricConfigs([]*chrome.TestConn{tconn})...)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(cleanUpRecorderCtx)

	meetTimeout := 50 * time.Second
	if roomSize == NoRoom {
		meetTimeout = 70 * time.Second
	} else if tier == "plus" {
		meetTimeout = 140 * time.Second
	} else if tier == "premium" {
		meetTimeout = 3 * time.Minute
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

		if roomSize != NoRoom {
			// Only premium tier need to change background to blur at the beginning.
			toBlur := tier == "premium"
			if err := conf.Join(ctx, inviteLink, toBlur); err != nil {
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
		}

		// Plus and premium tier.
		if tier == "plus" || tier == "premium" {
			application := googleSlides
			if tier == "premium" {
				application = googleDocs
			}
			if err := conf.Presenting(ctx, application); err != nil {
				return err
			}
		}

		// Premium tier.
		if roomSize != NoRoom && tier == "premium" {
			if err := conf.BackgroundChange(ctx); err != nil {
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
