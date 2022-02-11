// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
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
		Func:         CCAUIVideoOptionPerf,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Record video with different video option on CCA, measure UI performance including CPU usage",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Timeout:      20 * time.Minute,
		Fixture:      "ccaTestBridgeReady",
	})
}

// Candidates of bitrate multiplier to be tested.
// x2 is the default multiplier in chrome.
// x8 is the multiplier aligned with Ipad.
var multiplierCandidates = []int{2, 4, 6, 8, 10}

// Duration to wait for CPU to stabalize.
const stabilizationDuration time.Duration = 5 * time.Second

func CCAUIVideoOptionPerf(ctx context.Context, s *testing.State) {
	startApp := s.FixtValue().(cca.FixtureData).StartApp
	stopApp := s.FixtValue().(cca.FixtureData).StopApp

	cleanUpBenchmark, err := mediacpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark: ", err)
	}
	defer cleanUpBenchmark(ctx)

	if err := cpu.WaitUntilIdle(ctx); err != nil {
		s.Fatal("Failed to wait CPU idle: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	app, err := startApp(ctx)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(cleanupCtx context.Context) {
		if err := stopApp(cleanupCtx, s.HasError()); err != nil {
			s.Fatal("Failed to close CCA: ", err)
		}
	}(cleanupCtx)

	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Fatal("Failed to switch to video mode: ", err)
	}
	if err := app.EnableExpertMode(ctx); err != nil {
		s.Fatal("Failed to toggle expert mode: ", err)
	}

	if err := cca.MainMenu.Open(ctx, app); err != nil {
		s.Fatal("Failed to open setting menu: ", err)
	}
	defer cca.MainMenu.Close(ctx, app)

	if err := cca.ExpertMenu.Open(ctx, app); err != nil {
		s.Fatal("Failed to open expert setting menu: ", err)
	}
	defer cca.ExpertMenu.Close(ctx, app)

	if toggled, err := app.ToggleOption(ctx, cca.CustomVideoParametersOption); err != nil {
		s.Fatal("Failed to toggle custom video parameters: ", err)
	} else if !toggled {
		s.Fatal("Custom video parameters is not toggled")
	}

	perfValues := perf.NewValues()

	if err := app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		for _, profile := range []cca.Profile{cca.ProfileH264Baseline, cca.ProfileH264Main, cca.ProfileH264High} {
			// Set encoder profile.
			if err := app.SelectOption(ctx, cca.VideoProfileSelect, profile.Option()); err != nil {
				return err
			}

			// Get valid bitrate multiplier range and try all multiplier candidates.
			r, err := app.InputRange(ctx, cca.BitrateMultiplierRangeInput)
			if err != nil {
				return err
			}
			s.Logf("Camera facing %v, bitrate multiplier range: [%v, %v]", facing, r.Min, r.Max)
			for _, c := range multiplierCandidates {
				if c > r.Max || c < r.Min {
					s.Log("Skip unsuppported multiplier: ", c)
					continue
				}
				if err := app.SetRangeInput(ctx, cca.BitrateMultiplierRangeInput, c); err != nil {
					return err
				}

				// Record video and measure cpu usage.
				start, err := app.StartRecording(ctx, cca.TimerOff)
				if err != nil {
					return err
				}
				testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
				if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
					return errors.Wrap(err, "failed to sleep for CPU usage to stabilize")
				}
				usage, err := mediacpu.MeasureUsage(ctx, 15*time.Second)
				if err != nil {
					return errors.Wrap(err, "failed to measure cpu usage")
				}
				file, _, err := app.StopRecording(ctx, cca.TimerOff, start)
				if err != nil {
					return err
				}
				path, err := app.FilePathInSavedDir(ctx, file.Name())
				if err != nil {
					return err
				}
				if err := cca.CheckVideoProfile(path, profile); err != nil {
					return err
				}
				testing.ContextLogf(ctx, "Video perf profile=%v multiplier=%v: %v", profile.Name, c, usage)
				if cpu, ok := usage["cpu"]; ok {
					cpuMetric := fmt.Sprintf("cpu_usage_video_option-facing-%s-profile-%s-bitrate-x%d", facing, profile.Name, c)
					perfValues.Set(perf.Metric{
						Name:      cpuMetric,
						Unit:      "percent",
						Direction: perf.SmallerIsBetter,
					}, cpu)
				}
				if power, ok := usage["power"]; ok {
					powerMetric := fmt.Sprintf("power_usage_video_option-facing-%s-profile-%s-bitrate-x%d", facing, profile.Name, c)
					perfValues.Set(perf.Metric{
						Name:      powerMetric,
						Unit:      "Watts",
						Direction: perf.SmallerIsBetter,
					}, power)
				}
			}
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to record video though cameras: ", err)
	}

	// TODO(b/151047420): Collect ui latency data.

	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed to save perf metrics: ", err)
	}
}
