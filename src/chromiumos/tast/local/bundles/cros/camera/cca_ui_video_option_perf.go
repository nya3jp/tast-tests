// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"time"

	"github.com/abema/go-mp4"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/media/cpu"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIVideoOptionPerf,
		Desc:         "Record video with different video option on CCA, measure UI performance including CPU usage",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      10 * time.Minute,
		Pre:          chrome.LoggedIn(),
	})
}

// Candidates of bitrate multiplier to be tested.
// x2 is the default multiplier in chrome.
// x8 is the multiplier aligned with Ipad.
var multiplierCandidates = []int{2, 4, 6, 8, 10}

// Duration to wait for CPU to stabalize.
const stabilizationDuration time.Duration = 5 * time.Second

func CCAUIVideoOptionPerf(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	cleanUpBenchmark, err := cpu.SetUpBenchmark(ctx)
	if err != nil {
		s.Fatal("Failed to set up benchmark: ", err)
	}
	defer cleanUpBenchmark(ctx)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Fatal("Failed to close CCA: ", err)
		}
	}(ctx)

	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Fatal("Failed to switch to video mode: ", err)
	}
	if toggled, err := app.ToggleExpertMode(ctx); err != nil {
		s.Fatal("Failed to toggle expert mode: ", err)
	} else if !toggled {
		s.Fatal("Expert mode is not toggled")
	}
	if toggled, err := app.ToggleCustomVideoParameters(ctx); err != nil {
		s.Fatal("Failed to toggle custom video parameters: ", err)
	} else if !toggled {
		s.Fatal("Custom video parameters is not toggled")
	}

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
				if err := cpu.WaitUntilIdle(ctx); err != nil {
					return errors.Wrap(err, "failed to idle")
				}
				testing.ContextLog(ctx, "Sleeping to wait for CPU usage to stabilize for ", stabilizationDuration)
				if err := testing.Sleep(ctx, stabilizationDuration); err != nil {
					return errors.Wrap(err, "failed to sleep for CPU usage to stabilize")
				}
				start, err := app.StartRecording(ctx, cca.TimerOff)
				if err != nil {
					return err
				}
				usage, err := cpu.MeasureUsage(ctx, 15*time.Second)
				if err != nil {
					return errors.Wrap(err, "failed to measure cpu usage")
				}
				file, _, err := app.StopRecording(ctx, cca.TimerOff, start)
				if err != nil {
					return err
				}
				path, err := app.FilePathInSavedDirs(ctx, file.Name())
				if err != nil {
					return err
				}
				if err := checkVideoFile(path, profile); err != nil {
					return err
				}
				testing.ContextLogf(ctx, "Video perf profile=%v multiplier=%v: %v", profile, c, usage)
				// TODO(b/151047420): Dashboarding usage data with chromiumos/tast/common/perf.
			}
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to record video though cameras: ", err)
	}

	// TODO(b/151047420): Collect ui latency data.
}

func checkVideoFile(path string, profile cca.Profile) error {
	videoAVCConfigure := func(path string) (*mp4.AVCDecoderConfiguration, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open video file %v", path)
		}
		defer file.Close()
		boxes, err := mp4.ExtractBoxWithPayload(
			file, nil,
			mp4.BoxPath{
				mp4.BoxTypeMoov(),
				mp4.BoxTypeTrak(),
				mp4.BoxTypeMdia(),
				mp4.BoxTypeMinf(),
				mp4.BoxTypeStbl(),
				mp4.BoxTypeStsd(),
				mp4.StrToBoxType("avc1"),
				mp4.StrToBoxType("avcC"),
			})
		if err != nil {
			return nil, err
		}
		if len(boxes) != 1 {
			return nil, errors.Errorf("mp4 file %v has %v avcC box(es), want 1", path, len(boxes))
		}
		return boxes[0].Payload.(*mp4.AVCDecoderConfiguration), nil
	}

	config, err := videoAVCConfigure(path)
	if err != nil {
		return errors.Wrap(err, "failed to get videoAVCConfigure from result video")
	}
	if int(config.Profile) != int(profile) {
		return errors.Errorf("mismatch video profile, got %v; want %v", config.Profile, profile)
	}
	return nil
}
