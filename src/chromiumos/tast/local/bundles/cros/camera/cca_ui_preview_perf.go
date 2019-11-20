// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreviewPerf,
		Desc:         "Opens CCA and measures the CPU usage",
		Contacts:     []string{"shik@chromium.org", "kelsey.deuth@intel.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})

}

// CCAUIPreviewPerf launches the Chrome Camera App, waits for camera preview, fullscreens the
// application and starts measuring system CPU usage.
func CCAUIPreviewPerf(ctx context.Context, s *testing.State) {
	// Duration to wait for CPU to stabalize.
	const stabilizationDuration = 5 * time.Second
	// Duration of the interval during which CPU usage will be measured.
	const measureDuration = 20 * time.Second
	// Sample count for power measurement. The test will sleep for one second between each sample.
	const powerMeasureSampleCount = 10

	cr := s.PreValue().(*chrome.Chrome)

	// Prevents the CPU usage measurements from being affected by any previous tests.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to idle: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
	s.Log("Preview started")

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark mode: ", err)
	}
	defer cleanUpBenchmark(ctx)

	cpuUsage, err := measureCPUUsage(ctx, app, measureDuration, stabilizationDuration)
	if err != nil {
		s.Fatal("Failed in measureCPUUsage(): ", err)
	}
	s.Log("Measured CPU usage: ", cpuUsage)

	if err := saveMetric(perf.Metric{
		Name:      "cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage, s.OutDir()); err != nil {
		s.Error("Failed to save CPU result: ", err)
	}

	testing.ContextLogf(ctx, "Measuring power usage for %ds", powerMeasureSampleCount)
	totalPowerUsage := 0.0
	for i := 0; i < powerMeasureSampleCount; i++ {
		status, err := power.GetStatus(ctx)
		if err != nil {
			s.Fatal("Failed to get power status: ", err)
		}
		totalPowerUsage += status.BatteryEnergyRate

		// Sleep one second between each samples.
		if err := testing.Sleep(ctx, time.Second); err != nil {
			s.Error("Failed to sleep during power measurement: ", err)
			break
		}
	}
	avgPowerUsage := totalPowerUsage / float64(powerMeasureSampleCount)
	testing.ContextLogf(ctx, "Measured power usage: %f", avgPowerUsage)

	if err := saveMetric(perf.Metric{
		Name:      "avg_power_usage",
		Unit:      "W",
		Direction: perf.SmallerIsBetter,
	}, avgPowerUsage, s.OutDir()); err != nil {
		s.Error("Failed to save power result: ", err)
	}
}

// saveMetric saves the |metric| and |value| to |dir|.
func saveMetric(metric perf.Metric, value float64, dir string) error {
	pv := perf.NewValues()
	pv.Set(metric, value)
	return pv.Save(dir)
}

// measureCPUUsage fullscreens the camera preview, starts measuring the CPU usage, and returns the percentage of the CPU used.
func measureCPUUsage(ctx context.Context, app *cca.App, measureDuration, stabilizationDuration time.Duration) (float64, error) {
	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return 0, errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
	if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
		return 0, errors.Wrap(err, "failed to wait for CPU usage to stabilize")
	}

	testing.ContextLog(ctx, "Measuring CPU usage for ", measureDuration)
	cpuUsage, err := cpu.MeasureCPUUsage(ctx, measureDuration)
	if err != nil {
		return 0, errors.Wrap(err, "failed to measure CPU usage")
	}
	return cpuUsage, nil
}
