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

type perfEvent struct {
	Event    string  `json:"event"`
	Duration float64 `json:"duration"`
}

// MeasurementOptions contains the information for performance measurement.
type MeasurementOptions struct {
	IsColdStart              bool
	OutputDir                string
	ShouldMeasureUIBehaviors bool
}

// MeasurePerformance measures performance for CCA.
func MeasurePerformance(ctx context.Context, cr *chrome.Chrome, scripts []string, options MeasurementOptions) error {
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

	app, err := Init(ctx, cr, scripts, func(tconn *chrome.Conn) error {
		if err := setupPerfListener(ctx, tconn, options.IsColdStart); err != nil {
			return err
		}

		launchApp := fmt.Sprintf(`tast.promisify(chrome.management.launchApp)(%q);`, ID)
		if err := tconn.EvalPromise(ctx, launchApp, nil); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	if options.ShouldMeasureUIBehaviors {
		if err := measureUIBehaviors(ctx, cr, app, options.OutputDir); err != nil {
			return errors.Wrap(err, "failed to measure UI behaviors")
		}
	}

	if err := app.CollectPerfEvents(ctx, options.OutputDir); err != nil {
		return errors.Wrap(err, "failed to collect perf events")
	}

	return nil
}

// measureUIBehaviors measures the performance of UI behaviors such as taking picture, recording
// video, etc.
func measureUIBehaviors(ctx context.Context, cr *chrome.Chrome, app *App, outputDir string) error {
	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	return app.RunThroughCameras(ctx, func(facing Facing) error {
		if err := measureStreamingPerformance(ctx, cr, app, outputDir, false); err != nil {
			return errors.Wrap(err, "failed to measure preview performance")
		}

		if err := measureStreamingPerformance(ctx, cr, app, outputDir, true); err != nil {
			return errors.Wrap(err, "failed to measure performance for recording video")
		}

		if err := measureTakingPicturePerformance(ctx, app); err != nil {
			return errors.Wrap(err, "failed to measure performance for taking picture")
		}

		return nil
	})
}

// measureStreamingPerformance measures the CPU usage when streaming.
func measureStreamingPerformance(ctx context.Context, cr *chrome.Chrome, app *App, outputDir string, isRecording bool) error {
	// Duration of the interval during which CPU usage will be measured.
	const measureDuration = 20 * time.Second

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
	var CPUMetricName string
	if isRecording {
		CPUMetricName = "cpu_usage_recording"
	} else {
		CPUMetricName = "cpu_usage_preview"
	}
	if err := saveMetric(perf.Metric{
		Name:      CPUMetricName,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage, outputDir); err != nil {
		return errors.Wrap(err, "failed to save cpu usage result")
	}

	return nil
}

// measureTakingPicturePerformance takes a picture and measure the performance of UI operations.
func measureTakingPicturePerformance(ctx context.Context, app *App) error {
	if err := app.WaitForVideoActive(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switching to correct mode")
	if err := app.SwitchMode(ctx, Photo); err != nil {
		return err
	}

	if _, err := app.TakeSinglePhoto(ctx, TimerOff); err != nil {
		return err
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
func setupPerfListener(ctx context.Context, tconn *chrome.Conn, isColdStart bool) error {
	var launchEventName string
	if isColdStart {
		launchEventName = "launching-from-launch-app-cold"
	} else {
		launchEventName = "launching-from-launch-app-warm"
	}

	addPerfListener := fmt.Sprintf(`
		// Declared variables if not declared to avoid redeclaration error.
		if (typeof perfEvents === 'undefined') {
			var perfEvents;
		}
		if (typeof port === 'undefined') {
			var port;
		}

		perfEvents = [];
		port = chrome.runtime.connect(%q, {name: 'SET_PERF_CONNECTION'});
		port.onMessage.addListener((message) => {
		  perfEvents.push(message);
		});
		port.postMessage({name: %q});
	`, ID, launchEventName)
	if err := tconn.Exec(ctx, addPerfListener); err != nil {
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

	var events []perfEvent
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
