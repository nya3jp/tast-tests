// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/common/perf"
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
	perfValues := perf.NewValues()

	// App launch tests.
	const appLaunchTestTimeout = 90 * time.Second
	resetChrome := s.FixtValue().(cca.FixtureData).ResetChrome
	startApp := s.FixtValue().(cca.FixtureData).StartApp
	stopApp := s.FixtValue().(cca.FixtureData).StopApp
	appLaunchTestCtx, cancel := context.WithTimeout(ctx, appLaunchTestTimeout)
	defer cancel()

	if err := preparePerfTest(appLaunchTestCtx, resetChrome, func(ctx context.Context) (retErr error) {
		defer func(ctx context.Context) {
			if retErr != nil {
				resetChrome(ctx)
			}
		}(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()

		for _, perfEntryName := range []string{"launching-from-launch-app-cold", "launching-from-launch-app-warm"} {
			app, err := startApp(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to open CCA")
			}
			if err := app.CollectPerfEvents(ctx, perfValues, regexp.MustCompile(perfEntryName)); err != nil {
				return errors.Wrap(err, "failed to collect perf events")
			}
			if err := stopApp(ctx, retErr != nil); err != nil {
				retErr = errors.Wrap(retErr, err.Error())
			}
		}
		return nil
	}); err != nil {
		s.Error("Failed to run app launch test: ", err)
	}

	// UI tests.
	const defaultTimeout = 90 * time.Second
	const previewTestTimeout = 3 * time.Minute
	runTestWithApp := s.FixtValue().(cca.FixtureData).RunSubTest

	for _, tst := range []struct {
		name                 string
		testFunc             func(context.Context, *cca.App, *perf.Values) error
		perfEntryNamePattern *regexp.Regexp
		timeout              time.Duration
	}{{
		"testPreviewPerformance",
		testPreviewPerformance,
		regexp.MustCompile("launching-from-window-creation|camera-switching|mode-switching"),
		previewTestTimeout,
	}, {
		"testRecordingPerformance",
		testRecordingPerformance,
		regexp.MustCompile("^video-capture|camera-switching|mode-switching"),
		defaultTimeout,
	}, {
		"testTakingPicturePerformance",
		testTakingPicturePerformance,
		regexp.MustCompile("^photo-(capture|taking)|camera-switching|mode-switching"),
		defaultTimeout,
	}, {
		"testGifRecordingPerformance",
		testGifRecordingPerformance,
		regexp.MustCompile("^gif-capture|mode-switching"),
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

					if err := tst.testFunc(ctx, app, perfValues); err != nil {
						return err
					}
					if err := app.CollectPerfEvents(ctx, perfValues, tst.perfEntryNamePattern); err != nil {
						return errors.Wrap(err, "failed to collect perf events")
					}

					return nil
				}, cca.SubTestParams{})
			}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tst.name, err)
			}
		})
		cancel()
	}

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf metrics: ", err)
	}
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
	defer func(ctx context.Context) {
		cleanUpBenchmark(ctx)
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Prevents the CPU usage measurements from being affected by any previous tests.
	if err := cpu.WaitUntilIdle(ctx); err != nil {
		return errors.Wrap(err, "failed to idle")
	}

	if err := testBody(ctx); err != nil {
		return err
	}

	return nil
}

func testPreviewPerformance(ctx context.Context, app *cca.App, perfValues *perf.Values) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		return cca.MeasurePreviewPerformance(ctx, app, perfValues, facing)
	})
}

func testRecordingPerformance(ctx context.Context, app *cca.App, perfValues *perf.Values) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		return cca.MeasureRecordingPerformance(ctx, app, perfValues, facing)
	})
}

func testTakingPicturePerformance(ctx context.Context, app *cca.App, perfValues *perf.Values) error {
	return app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		return cca.MeasureTakingPicturePerformance(ctx, app)
	})
}

func testGifRecordingPerformance(ctx context.Context, app *cca.App, perfValues *perf.Values) error {
	return cca.MeasureGifRecordingPerformance(ctx, app)
}
