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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/testutil"
	mediacpu "chromiumos/tast/local/media/cpu"
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

// measureStablizedUsage measures the CPU and power usage after it's cooled down for stabilizationDuration.
func measureStablizedUsage(ctx context.Context) (map[string]float64, error) {
	testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return nil, errors.Wrap(err, "failed to wait for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage for ", measureDuration)
	return mediacpu.MeasureUsage(ctx, measureDuration)
}

// MeasurePreviewPerformance measures the performance of preview with QR code detection on and off.
func MeasurePreviewPerformance(ctx context.Context, app *App, perfValues *perf.Values, facing Facing) error {
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
	if err := app.EnableQRCodeDetection(ctx); err != nil {
		return errors.Wrap(err, "failed to ensure QR code detection is enabled")
	}

	usageQR, err := measureStablizedUsage(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to measure CPU and power usage with QR code detection")
	}

	if err := app.DisableQRCodeDetection(ctx); err != nil {
		return errors.Wrap(err, "failed to ensure QR code detection is disabled")
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

// MeasureRecordingPerformance measures the performance of video recording.
func MeasureRecordingPerformance(ctx context.Context, app *App, perfValues *perf.Values, facing Facing) error {
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

// MeasureTakingPicturePerformance takes a picture and measure the performance of UI operations.
func MeasureTakingPicturePerformance(ctx context.Context, app *App) error {
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

// MeasureGifRecordingPerformance records a gif and measure the performance of UI operations.
func MeasureGifRecordingPerformance(ctx context.Context, app *App) error {
	if err := app.WaitForVideoActive(ctx); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Switching to video mode")
	if err := app.SwitchMode(ctx, Video); err != nil {
		return err
	}
	if _, err := app.RecordGif(ctx, true); err != nil {
		return err
	}

	return nil
}

// CollectPerfEvents collects perf events matching |perfEntryPattern| from launch until now and saves them into given place.
func (a *App) CollectPerfEvents(ctx context.Context, perfValues *perf.Values, perfEntryPattern *regexp.Regexp) error {
	allEntries, err := a.appWindow.Perfs(ctx)
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

	var entries []testutil.PerfEntry
	for _, entry := range allEntries {
		name := informativeEventName(entry)
		if perfEntryPattern.MatchString(name) {
			entries = append(entries, entry)
		}
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
