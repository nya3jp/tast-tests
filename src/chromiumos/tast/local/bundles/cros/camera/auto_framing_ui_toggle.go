// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/features"
	"chromiumos/tast/local/camera/histogramutil"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AutoFramingUIToggle,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks toggling Auto-framing from UI works",
		Contacts:     []string{"kamesan@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"camera_feature_auto_framing", "chrome", caps.BuiltinCamera},
		Fixture:      "ccaTestBridgeReadyWithAutoFramingForceEnabled",
		Timeout:      7 * time.Minute,
	})
}

func AutoFramingUIToggle(ctx context.Context, s *testing.State) {
	model, err := crosconfig.Get(ctx, "/", "name")
	if err != nil {
		s.Fatal("Failed to get device model: ", err)
	}
	if modelConf, err := features.NewModelConfig(model); err == nil {
		conf := features.NewFeatureConfig()
		if err := modelConf.FeatureConfig("auto_framing", conf); err != nil {
			s.Fatal("Failed to get feature config: ", err)
		}
		s.Log("Feature config: ", conf)
	} else {
		// Put an empty feature config for Auto Framing to work.
		const overrideConfigFilePath = "/run/camera/auto_framing_config.json"
		if err := features.WriteFeatureConfig(ctx, features.NewFeatureConfig(), overrideConfigFilePath, true); err != nil {
			s.Fatalf("Failed to write feature config to %v: %v", overrideConfigFilePath, err)
		}
		s.Log("Wrote empty feature config to ", overrideConfigFilePath)
		defer func() {
			if err := os.Remove(overrideConfigFilePath); err != nil {
				s.Errorf("Failed to remove %v: %v", overrideConfigFilePath, err)
			}
		}()
	}

	cr := s.FixtValue().(cca.FixtureData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to establish connection to the test API extension")
	}

	type Action struct {
		name string
		run  func(ctx context.Context, app *cca.App) error
	}
	sleep := func(d time.Duration) Action {
		return Action{
			name: "sleep",
			run: func(ctx context.Context, app *cca.App) error {
				return testing.Sleep(ctx, d)
			},
		}
	}
	toggleFraming := func(enable bool) Action {
		return Action{
			name: "toggle \"Camera framing\" in Quick Settings",
			run: func(ctx context.Context, app *cca.App) error {
				return quicksettings.ToggleSetting(ctx, tconn, quicksettings.SettingPodCameraFraming, enable)
			},
		}
	}
	ccaTakePhotoNTimes := func(n int) Action {
		return Action{
			name: "take photo in CCA",
			run: func(ctx context.Context, app *cca.App) error {
				for i := 0; i < n; i++ {
					if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
						return errors.Wrapf(err, "failed to take %v-th photo", i)
					}
				}
				return nil
			},
		}
	}
	var ccaRecordingStartTime time.Time
	ccaStartRecording := func() Action {
		return Action{
			name: "start recording video in CCA",
			run: func(ctx context.Context, app *cca.App) error {
				ccaRecordingStartTime, err = app.StartRecording(ctx, cca.TimerOff)
				return err
			},
		}
	}
	ccaStopRecording := func() Action {
		return Action{
			name: "stop recording video in CCA",
			run: func(ctx context.Context, app *cca.App) error {
				_, _, err := app.StopRecording(ctx, cca.TimerOff, ccaRecordingStartTime)
				return err
			},
		}
	}

	const (
		autoFramingNoError  = 0.0
		minDetectionLatency = 1e3
		enabledTimeMargin   = 10.0
	)
	histogramTests := func(enabledCount int, enabledTime float64) histogramutil.HistogramTests {
		return histogramutil.HistogramTests{
			"ChromeOS.Camera.AutoFraming.AverageDetectionLatency": histogramutil.AssertHistogramMeanGt(minDetectionLatency),
			"ChromeOS.Camera.AutoFraming.DetectionHitRate":        histogramutil.AssertHistogramInRange(0.0, 100.0),
			"ChromeOS.Camera.AutoFraming.EnabledCount":            histogramutil.AssertHistogramEq(float64(enabledCount)),
			"ChromeOS.Camera.AutoFraming.EnabledTime":             histogramutil.AssertHistogramInRange(enabledTime-enabledTimeMargin, enabledTime+enabledTimeMargin),
			"ChromeOS.Camera.AutoFraming.Error":                   histogramutil.AssertHistogramEq(autoFramingNoError),
		}
	}

	runTestWithApp := s.FixtValue().(cca.FixtureData).RunTestWithApp
	subTestTimeout := 120 * time.Second
	actionInterval := 4 * time.Second
	for _, tc := range []struct {
		name           string
		mode           cca.Mode
		actions        []Action
		histogramTests histogramutil.HistogramTests
	}{
		{
			// Warm-up session to stabilize the timing checks in the following tests.
			name: "warmUp",
			mode: cca.Photo,
			actions: []Action{
				toggleFraming(true),
				sleep(4 * time.Second),
			},
			histogramTests: nil,
		},
		{
			name: "testTogglingFraming",
			mode: cca.Photo,
			actions: []Action{
				toggleFraming(true),
				sleep(actionInterval),
				toggleFraming(false),
				sleep(actionInterval),
				toggleFraming(true),
				toggleFraming(false),
				sleep(actionInterval),
				toggleFraming(true),
				toggleFraming(false),
				toggleFraming(true),
				sleep(actionInterval),
				toggleFraming(false),
				toggleFraming(true),
				toggleFraming(false),
				toggleFraming(true),
				sleep(actionInterval),
			},
			// Quick clicks are debounced to at most one ON <-> OFF toggling.
			histogramTests: histogramTests(3, 60.0),
		},
		{
			name: "testPhotoTaking",
			mode: cca.Photo,
			actions: []Action{
				toggleFraming(true),
				sleep(actionInterval),
				ccaTakePhotoNTimes(1),
				toggleFraming(false),
				sleep(actionInterval),
				ccaTakePhotoNTimes(2),
				toggleFraming(true),
				sleep(actionInterval),
				ccaTakePhotoNTimes(3),
				toggleFraming(false),
				sleep(actionInterval),
				ccaTakePhotoNTimes(2),
				toggleFraming(true),
				sleep(actionInterval),
				ccaTakePhotoNTimes(1),
			},
			histogramTests: histogramTests(3, 60.0),
		},
		{
			name: "testVideoRecording",
			mode: cca.Video,
			actions: []Action{
				toggleFraming(true),
				ccaStartRecording(),
				sleep(actionInterval),
				toggleFraming(false),
				sleep(actionInterval),
				ccaStopRecording(),
				sleep(actionInterval),
				ccaStartRecording(),
				sleep(actionInterval),
				toggleFraming(true),
				sleep(actionInterval),
				ccaStopRecording(),
			},
			histogramTests: histogramTests(2, 40.0),
		},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
				if err := app.SwitchMode(ctx, tc.mode); err != nil {
					return errors.Wrap(err, "failed to switch mode in CCA")
				}
				recorder, err := tc.histogramTests.Record(ctx, tconn)
				if err != nil {
					return errors.Wrap(err, "failed to start histogram recorder")
				}
				if err := quicksettings.Expand(ctx, tconn); err != nil {
					return errors.Wrap(err, "failed to expand Quick Settings")
				}
				for _, a := range tc.actions {
					if err := a.run(ctx, app); err != nil {
						return errors.Wrapf(err, "failed to %v", a.name)
					}
				}
				if err = app.Close(ctx); err != nil {
					return errors.Wrap(err, "failed to close Chrome Camera App")
				}
				if tc.histogramTests != nil {
					return tc.histogramTests.Verify(ctx, tconn, recorder)
				}
				return nil
			}, cca.TestWithAppParams{StopAppOnlyIfExist: true}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tc.name, err)
			}
		})
		cancel()
	}
}
