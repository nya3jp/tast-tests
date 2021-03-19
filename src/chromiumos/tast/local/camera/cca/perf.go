// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cca provides utilities to interact with Chrome Camera App.
package cca

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

const (
	// Time reserved for cleanup.
	cleanupTime = 10 * time.Second

	// Duration to wait for CPU to stabalize.
	stabilizationDuration time.Duration = 5 * time.Second

	// Duration of the interval during which CPU usage will be measured for streaming.
	measureDuration = 20 * time.Second
)

// MeasurementOptions contains the information for performance measurement.
type MeasurementOptions struct {
	PerfValues               *perf.Values
	ShouldMeasureUIBehaviors bool
	OutputDir                string
}

// MeasurePerformance measures performance for CCA.
func MeasurePerformance(ctx context.Context, startApp StartAppFunc, stopApp StopAppFunc, options MeasurementOptions) (retErr error) {
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

	app, err := startApp(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open CCA")
	}
	defer func(ctx context.Context) {
		if err := stopApp(ctx, retErr != nil); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
	}(ctx)

	if options.ShouldMeasureUIBehaviors {
		if err := measureUIBehaviors(ctx, app, options.PerfValues); err != nil {
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
func measureUIBehaviors(ctx context.Context, app *App, perfValues *perf.Values) error {
	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	return app.RunThroughCameras(ctx, func(facing Facing) error {
		if err := measurePreviewPerformance(ctx, app, perfValues, facing); err != nil {
			return errors.Wrap(err, "failed to measure preview performance")
		}

		if err := measureRecordingPerformance(ctx, app, perfValues, facing); err != nil {
			return errors.Wrap(err, "failed to measure video recording performance")
		}

		if err := measureTakingPicturePerformance(ctx, app); err != nil {
			return errors.Wrap(err, "failed to measure performance for taking picture")
		}

		return nil
	})
}

// measureStablizedUsage measures the CPU and power usage after it's cooled down for stabilizationDuration.
func measureStablizedUsage(ctx context.Context) (map[string]float64, error) {
	testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return nil, errors.Wrap(err, "failed to wait for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage for ", measureDuration)
	return cpu.MeasureUsage(ctx, measureDuration)
}

// measurePreviewPerformance measures the performance of preview with QR code detection on and off.
func measurePreviewPerformance(ctx context.Context, app *App, perfValues *perf.Values, facing Facing) error {
	testing.ContextLog(ctx, "Switching to photo mode")
	if err := app.SwitchMode(ctx, Photo); err != nil {
		return errors.Wrap(err, "failed to switch to photo mode")
	}

	scanBarcode, err := app.GetState(ctx, "enable-scan-barcode")
	if err != nil {
		return errors.Wrap(err, "failed to check barcode state")
	}
	if scanBarcode {
		return errors.New("QR code detection should be off by default")
	}

	usage, err := measureStablizedUsage(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU and power usage")
	}

	var cpuUsage float64
	if cpuUsage, exist := usage["cpu"]; exist {
		testing.ContextLogf(ctx, "Measured preview CPU usage: %.1f%%", cpuUsage)

		perfValues.Set(perf.Metric{
			Name:      fmt.Sprintf("cpu_usage_preview-facing-%s", facing),
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, cpuUsage)
	} else {
		testing.ContextLog(ctx, "Failed to measure preview CPU usage")
	}

	var powerUsage float64
	if powerUsage, exist := usage["power"]; exist {
		testing.ContextLogf(ctx, "Measured preview power usage: %.1f Watts", powerUsage)

		perfValues.Set(perf.Metric{
			Name:      fmt.Sprintf("power_usage_preview-facing-%s", facing),
			Unit:      "Watts",
			Direction: perf.SmallerIsBetter,
		}, powerUsage)
	} else {
		testing.ContextLog(ctx, "Failed to measure preview power usage")
	}

	// Enable QR code detection and measure the performance again.
	enabled, err := app.ToggleQRCodeOption(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to enable QR code detection")
	}
	if !enabled {
		return errors.Wrap(err, "QR code detection is not enabled after toggling")
	}

	usageQR, err := measureStablizedUsage(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU and power usage with QR code detection")
	}

	enabled, err = app.ToggleQRCodeOption(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to disable QR code detection")
	}
	if enabled {
		return errors.Wrap(err, "QR code detection is not disabled after toggling")
	}

	if cpuUsageQR, exist := usageQR["cpu"]; exist {
		perfValues.Set(perf.Metric{
			Name:      fmt.Sprintf("cpu_usage_qrcode-facing-%s", facing),
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, cpuUsageQR)

		if cpuUsage != 0 {
			overhead := math.Max(0, cpuUsageQR-cpuUsage)

			testing.ContextLogf(ctx, "Measured QR code detection CPU usage: %.1f%%, overhead = %.1f%%", cpuUsageQR, overhead)
			perfValues.Set(perf.Metric{
				Name:      fmt.Sprintf("cpu_overhead_qrcode-facing-%s", facing),
				Unit:      "percent",
				Direction: perf.SmallerIsBetter,
			}, overhead)
		}
	} else {
		testing.ContextLog(ctx, "Failed to measure preview CPU usage with QR code detection")
	}

	if powerUsageQR, exist := usageQR["power"]; exist {
		perfValues.Set(perf.Metric{
			Name:      fmt.Sprintf("power_usage_qrcode-facing-%s", facing),
			Unit:      "Watts",
			Direction: perf.SmallerIsBetter,
		}, powerUsageQR)

		if powerUsage != 0 {
			overhead := math.Max(0, powerUsageQR-powerUsage)

			testing.ContextLogf(ctx, "Measured QR code detection power usage: %.1f Watts, overhead = %.1f Watts", powerUsageQR, overhead)
			perfValues.Set(perf.Metric{
				Name:      fmt.Sprintf("power_overhead_qrcode-facing-%s", facing),
				Unit:      "Watts",
				Direction: perf.SmallerIsBetter,
			}, overhead)
		}
	} else {
		testing.ContextLog(ctx, "Failed to measure preview power usage with QR code detection")
	}
	return nil
}

// measureRecordingPerformance measures the performance of video recording.
func measureRecordingPerformance(ctx context.Context, app *App, perfValues *perf.Values, facing Facing) error {
	testing.ContextLog(ctx, "Switching to video mode")
	if err := app.SwitchMode(ctx, Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}

	recordingStartTime, err := app.StartRecording(ctx, TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to start recording for performance measurement")
	}

	usage, err := measureStablizedUsage(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU and power usage")
	}

	if _, _, err := app.StopRecording(ctx, false, recordingStartTime); err != nil {
		return errors.Wrap(err, "failed to stop recording for performance measurement")
	}

	if cpuUsage, exist := usage["cpu"]; exist {
		testing.ContextLogf(ctx, "Measured recording CPU usage: %.1f%%", cpuUsage)

		perfValues.Set(perf.Metric{
			Name:      fmt.Sprintf("cpu_usage_recording-facing-%s", facing),
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, cpuUsage)
	} else {
		testing.ContextLog(ctx, "Failed to measure recording CPU usage")
	}

	if powerUsage, exist := usage["power"]; exist {
		testing.ContextLogf(ctx, "Measured recording power usage: %.1f Watts", powerUsage)

		perfValues.Set(perf.Metric{
			Name:      fmt.Sprintf("power_usage_recording-facing-%s", facing),
			Unit:      "Watts",
			Direction: perf.SmallerIsBetter,
		}, powerUsage)
	} else {
		testing.ContextLog(ctx, "Failed to measure recording power usage")
	}
	return nil
}

// measureTakingPicturePerformance takes a picture and measure the performance of UI operations.
func measureTakingPicturePerformance(ctx context.Context, app *App) error {
	if err := app.WaitForVideoActive(ctx); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Switching to photo mode")
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
