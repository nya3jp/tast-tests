// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

// Duration to wait for CPU to stabalize.
const stabilizationDuration time.Duration = 5 * time.Second

// MeasurementOptions contains the information for performance measurement.
type MeasurementOptions struct {
	PerfValues               *perf.Values
	ShouldMeasureUIBehaviors bool
	OutputDir                string
}

// MeasurePerformance measures performance for CCA.
func MeasurePerformance(ctx context.Context, cr *chrome.Chrome, scripts []string, options MeasurementOptions, tb *testutil.TestBridge, useSWA bool) (retErr error) {
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

	app, err := New(ctx, cr, scripts, options.OutputDir, tb, useSWA)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to close CCA: ", err)
			} else {
				retErr = err
			}
		}
	}(ctx)

	if options.ShouldMeasureUIBehaviors {
		if err := measureUIBehaviors(ctx, cr, app, options.PerfValues); err != nil {
			return errors.Wrap(err, "failed to measure UI behaviors")
		}
	}

	if err := app.CollectPerfEvents(ctx, options.PerfValues); err != nil {
		return errors.Wrap(err, "failed to collect perf events")
	}

	return nil
}

// measureUIBehaviors measures the performance of UI behaviors such as taking picture, recording
// video, etc.
func measureUIBehaviors(ctx context.Context, cr *chrome.Chrome, app *App, perfValues *perf.Values) error {
	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	return app.RunThroughCameras(ctx, func(facing Facing) error {
		if err := measureStreamingPerformance(ctx, cr, app, perfValues, facing, false /* isRecording */); err != nil {
			return errors.Wrap(err, "failed to measure preview performance")
		}

		if err := measureStreamingPerformance(ctx, cr, app, perfValues, facing, true /* isRecording */); err != nil {
			return errors.Wrap(err, "failed to measure performance for recording video")
		}

		if err := measureTakingPicturePerformance(ctx, app); err != nil {
			return errors.Wrap(err, "failed to measure performance for taking picture")
		}

		return nil
	})
}

// measureStreamingPerformance measures the CPU usage when streaming.
func measureStreamingPerformance(ctx context.Context, cr *chrome.Chrome, app *App, perfValues *perf.Values, facing Facing, isRecording bool) error {
	// Duration of the interval during which CPU usage will be measured.
	const measureDuration = 20 * time.Second

	var mode Mode
	if isRecording {
		mode = Video
	} else {
		mode = Photo
	}
	testing.ContextLog(ctx, "Switching to correct mode")
	if err := app.SwitchMode(ctx, mode); err != nil {
		return errors.Wrap(err, "failed to switch to correct mode")
	}

	var recordingStartTime time.Time
	if isRecording {
		// Start the recording.
		var err error
		recordingStartTime, err = app.StartRecording(ctx, TimerOff)
		if err != nil {
			return errors.Wrap(err, "failed to start recording for performance measurement")
		}
	}

	testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return errors.Wrap(err, "failed to wait for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage for ", measureDuration)
	cpuUsage, err := cpu.MeasureCPUUsage(ctx, measureDuration)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU usage")
	}

	if isRecording {
		if _, _, err := app.StopRecording(ctx, false, recordingStartTime); err != nil {
			return errors.Wrap(err, "failed to stop recording for performance measurement")
		}
	}

	testing.ContextLog(ctx, "Measured cpu usage: ", cpuUsage)
	var CPUMetricNameBase string
	if isRecording {
		CPUMetricNameBase = "cpu_usage_recording"
	} else {
		CPUMetricNameBase = "cpu_usage_preview"
	}
	perfValues.Set(perf.Metric{
		Name:      fmt.Sprintf("%s-facing-%s", CPUMetricNameBase, facing),
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)

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

// CollectPerfEvents collects all perf events from launch until now and saves them into given place.
func (a *App) CollectPerfEvents(ctx context.Context, perfValues *perf.Values) error {
	entries, err := a.appWindow.Perfs(ctx)
	if err != nil {
		return err
	}

	informativeEventName := func(entry testutil.PerfEntry) string {
		perfInfo := entry.PerfInfo
		if len(perfInfo.Facing) > 0 {
			// To avoid containing invalid character in the metrics name, we should remove the non-Alphanumeric characters from the facing.
			// e.g. When the facing is not set, the corresponding string will be (not-set).
			reg := regexp.MustCompile("[^a-zA-Z0-9]+")
			validFacingString := reg.ReplaceAllString(perfInfo.Facing, "")
			return fmt.Sprintf(`%s-facing-%s`, entry.Event, validFacingString)
		}
		return entry.Event
	}

	countMap := make(map[string]int)
	for _, entry := range entries {
		countMap[informativeEventName(entry)]++
	}

	resultMap := make(map[string]float64)
	for _, entry := range entries {
		eventName := informativeEventName(entry)
		resultMap[eventName] += entry.Duration / float64(countMap[eventName])
	}

	for name, value := range resultMap {
		testing.ContextLogf(ctx, "Perf event: %s => %f ms", name, value)
		perfValues.Set(perf.Metric{
			Name:      name,
			Unit:      "milliseconds",
			Direction: perf.SmallerIsBetter,
		}, value)
	}
	return nil
}
