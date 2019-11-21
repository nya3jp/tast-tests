// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// Duration to wait for CPU to stabalize.
const stabilizationDuration time.Duration = 5 * time.Second

// PerfEvent contains the name of the perf event and its duration.
type PerfEvent struct {
	Event    string  `json:"event"`
	Duration float64 `json:"duration"`
}

// MeasurePerformance measures performance for CCA.
func MeasurePerformance(ctx context.Context, cr *chrome.Chrome, scripts []string, outputDir string, isRecording bool) error {
	// Duration of the interval during which CPU usage will be measured.
	const measureDuration = 20 * time.Second
	// Time reserved for cleanup.
	const cleanupTime = 10 * time.Second

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up benchmark")
	}
	defer cleanUpBenchmark(ctx)

	// Reserve time for cleanup at the end of the test.
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTime)
	defer cancel()

	// Prevents the CPU usage measurements from being affected by any previous tests.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to idle")
	}

	app, err := New(ctx, cr, scripts)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return errors.Wrap(err, "failed to wait for CPU usage to stabilize")
	}

	var recordingStartTime time.Time
	if isRecording {
		testing.ContextLog(ctx, "Switching to correct mode")
		if err := app.SwitchMode(ctx, Video); err != nil {
			return errors.Wrap(err, "failed to switch to correct mode")
		}

		// Start the recording.
		recordingStartTime = time.Now()
		if err := app.ClickShutter(ctx); err != nil {
			return errors.Wrap(err, "failed to start recording for performance measurement")
		}
	}

	testing.ContextLog(ctx, "Measuring CPU usage for ", measureDuration)
	cpuUsage, err := cpu.MeasureCPUUsage(ctx, measureDuration)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU usage")
	}

	if isRecording {
		// Stop the recording.
		if err := app.ClickShutter(ctx); err != nil {
			return errors.Wrap(err, "failed to stop recording for performance measurement")
		}

		dir, err := GetSavedDir(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "cannot get saved dir")
		}
		if _, err := app.WaitForFileSaved(ctx, dir, VideoPattern, recordingStartTime); err != nil {
			return errors.Wrap(err, "cannot find result video")
		}
		if err := app.WaitForState(ctx, "video-saving", false); err != nil {
			return errors.Wrap(err, "video saving hasn't ended")
		}
	}

	testing.ContextLog(ctx, "Measured cpu usage: ", cpuUsage)
	if err := saveMetric(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage, outputDir); err != nil {
		return errors.Wrap(err, "failed to save cpu usage result")
	}

	if err := app.CollectPerfEvents(ctx, outputDir); err != nil {
		return errors.Wrap(err, "failed to collect perf events")
	}

	return nil
}

// saveMetric saves the |metric| and |value| to |dir|.
func saveMetric(metric perf.Metric, value float64, dir string) error {
	pv := perf.NewValues()
	pv.Set(metric, value)
	return pv.Save(dir)
}

// setupPerfListener setups the connection to CCA and add a perf event listener.
func setupPerfListener(ctx context.Context, tconn *chrome.Conn) error {
	addPerfListener := fmt.Sprintf(`
		perfEvents = [];
		port = chrome.runtime.connect(%q, {name: 'SET_PERF_CONNECTION'});
		port.onMessage.addListener((message) => {
		  perfEvents.push(message);
		});
	`, ccaID)
	if err := tconn.Exec(ctx, addPerfListener); err != nil {
		return err
	}

	if err := tconn.Exec(ctx, "port.postMessage({name: 'launching-from-test'});"); err != nil {
		return err
	}

	return nil
}

// CollectPerfEvents collects all perf events from launch until now and saves them into given place.
func (a *App) CollectPerfEvents(ctx context.Context, outputDir string) error {
	tconn, err := a.cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	var events []PerfEvent
	if err := tconn.Eval(ctx, "perfEvents", &events); err != nil {
		return err
	}

	for _, event := range events {
		testing.ContextLogf(ctx, "Perf event: %s => %f ms", event.Event, event.Duration)
		if err := saveMetric(perf.Metric{
			Name:      event.Event,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, event.Duration, outputDir); err != nil {
			return errors.Wrap(err, "failed to save perf event")
		}
	}

	return nil
}
