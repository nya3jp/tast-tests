// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/cpu"
	mediacpu "chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Opens CCA and measures the UI performance including CPU and power usage",
		Contacts:     []string{"wtlee@chromium.org", "inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      8 * time.Minute,
		Fixture:      "ccaTestBridgeReady",
	})
}

// CCAUIPerf measure cold/warm start time of CCA and also measure its
// performance through some UI operations.
func CCAUIPerf(ctx context.Context, s *testing.State) {
	const defaultTimeout = 90 * time.Second
	perfData := cca.NewPerfData()
	resetChrome := s.FixtValue().(cca.FixtureData).ResetChrome

	// App launch tests.
	startApp := s.FixtValue().(cca.FixtureData).StartApp
	stopApp := s.FixtValue().(cca.FixtureData).StopApp
	appLaunchTestCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	s.Run(appLaunchTestCtx, "testAppLaunch", func(ctx context.Context, s *testing.State) {
		if err := testAppLaunch(ctx, resetChrome, startApp, stopApp, perfData); err != nil {
			s.Error("Failed to pass testAppLaunch subtest: ", err)
		}
	})
	cancel()

	// UI tests.
	const previewTestTimeout = 5 * time.Minute
	runTestWithApp := s.FixtValue().(cca.FixtureData).RunTestWithApp

	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *cca.App, *cca.PerfData) error
		timeout  time.Duration
	}{{
		"testPreviewPerformance",
		testPreviewPerformance,
		previewTestTimeout,
	}, {
		"testRecordingPerformance",
		testRecordingPerformance,
		defaultTimeout,
	}, {
		"testTakingPicturePerformance",
		testTakingPicturePerformance,
		defaultTimeout,
	}, {
		"testGifRecordingPerformance",
		testGifRecordingPerformance,
		defaultTimeout,
	}} {
		subTestCtx, cancel := context.WithTimeout(ctx, tst.timeout)
		s.Run(subTestCtx, tst.name, func(ctx context.Context, s *testing.State) {
			if err := preparePerfTest(ctx, resetChrome, func(ctx context.Context) error {
				return runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
					testing.ContextLog(ctx, "Fullscreening window")
					if err := app.FullscreenWindow(ctx); err != nil {
						return errors.Wrap(err, "failed to fullscreen window")
					}
					if err := app.WaitForVideoActive(ctx); err != nil {
						return errors.Wrap(err, "preview is inactive after fullscreening window")
					}

					if err := tst.testFunc(ctx, app, perfData); err != nil {
						return err
					}
					if err := app.CollectPerfEvents(ctx, perfData); err != nil {
						return errors.Wrap(err, "failed to collect perf events")
					}

					return nil
				}, cca.TestWithAppParams{})
			}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tst.name, err)
			}
		})
		cancel()
	}

	if err := perfData.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf metrics: ", err)
	}
}

func testAppLaunch(ctx context.Context, resetChrome cca.ResetChromeFunc,
	startApp cca.StartAppFunc, stopApp cca.StopAppFunc, perfData *cca.PerfData) error {
	return preparePerfTest(ctx, resetChrome, func(ctx context.Context) (retErr error) {
		// Open/close app twice to collect app launched from cold/warm start.
		for i := 0; i < 2; i++ {
			app, err := startApp(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to open CCA")
			}
			if err := app.CollectPerfEvents(ctx, perfData); err != nil {
				return errors.Wrap(err, "failed to collect perf events")
			}
			if err := stopApp(ctx, retErr != nil); err != nil {
				return errors.Wrap(retErr, err.Error())
			}
		}
		return nil
	})
}

func preparePerfTest(ctx context.Context, resetChrome cca.ResetChromeFunc, testBody func(ctx context.Context) error) error {
	// Reset chrome to clean cached web assembly compilation result.
	if err := resetChrome(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome before running performance test")
	}

	cleanUpBenchmark, err := mediacpu.SetUpBenchmark(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set up benchmark")
	}
	defer cleanUpBenchmark(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Prevents the CPU usage measurements from being affected by any previous tests.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to idle")
	}

	return testBody(ctx)
}

func testPreviewPerformance(ctx context.Context, app *cca.App, perfData *cca.PerfData) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		return cca.MeasurePreviewPerformance(ctx, app, perfData, facing)
	})
}

func testRecordingPerformance(ctx context.Context, app *cca.App, perfData *cca.PerfData) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		return cca.MeasureRecordingPerformance(ctx, app, perfData, facing)
	})
}

func testTakingPicturePerformance(ctx context.Context, app *cca.App, perfData *cca.PerfData) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		return cca.MeasureTakingPicturePerformance(ctx, app)
	})
}

func testGifRecordingPerformance(ctx context.Context, app *cca.App, perfData *cca.PerfData) error {
	// TODO(b/201335131): Measure performance of per camera facing test
	// without cached web assembly result.
	return cca.MeasureGifRecordingPerformance(ctx, app)
}
