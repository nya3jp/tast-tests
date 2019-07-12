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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreviewPerf,
		Desc:         "Opens CCA and measures the CPU utilization",
		Contacts:     []string{"kelsey.deuth@intel.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})

}

// CCAUIPreviewPerf launches the Chrome Camera App, waits for camera preview, fullscreens the
// application and starts measuring system CPU utilization.
func CCAUIPreviewPerf(ctx context.Context, s *testing.State) {
	const (
		// Duration of the interval during which CPU usage will be measured.
		measureDuration = 20 * time.Second
	)

	cr := s.PreValue().(*chrome.Chrome)

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

	restartApp := func() {
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
	}

	cpuUsage, err := measureUtilization(ctx, app, measureDuration)
	if err != nil {
		s.Error("Failed in testScreen(): ", err)
		restartApp()
	}
	s.Log("Measured CPU utilization: ", cpuUsage)

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	pv.Set(perf.Metric{
		Name:      "tast_cpu_usage",
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, cpuUsage)
}

func measureUtilization(ctx context.Context, app *cca.App, measureDuration time.Duration) (float64, error) {
	restore := func() error {
		if err := app.RestoreWindow(ctx); err != nil {
			return errors.Wrap(err, "failed to restore window")
		}
		if err := app.WaitForVideoActive(ctx); err != nil {
			return errors.Wrap(err, "preview is inactive after restoring window")
		}
		return nil
	}
	if err := restore(); err != nil {
		return 0, err
	}

	testing.ContextLog(ctx, "Fullscreening window")
	if err := app.FullscreenWindow(ctx); err != nil {
		return 0, errors.Wrap(err, "failed to fullscreen window")
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return 0, errors.Wrap(err, "preview is inactive after fullscreening window")
	}

	testing.ContextLog(ctx, "Measuring CPU utilization for ", measureDuration)
	cpuUsage, err := cpu.MeasureUsage(ctx, measureDuration)
	if err != nil {
		return 0, errors.Wrap(err, "Failed to measure CPU usage")
	}
	if err := restore(); err != nil {
		return 0, errors.Wrap(err, "failed in restore() after fullscreening window")
	}

	return cpuUsage, nil
}
