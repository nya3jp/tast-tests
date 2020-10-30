// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"time"

	"github.com/abema/go-mp4"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIRecordVideo,
		Desc:         "Opens CCA and verifies video recording related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Timeout:      5 * time.Minute,
		Pre:          chrome.LoggedIn(),
	})
}

// durationTolerance is the tolerate difference when comparing video duration.
const durationTolerance = 300 * time.Millisecond

func CCAUIRecordVideo(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, false)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDirs(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	for _, tc := range []struct {
		name  string
		run   func(context.Context, *cca.App) error
		timer cca.TimerState
	}{
		{"testRecordVideoWithWindowChanged", testRecordVideoWithWindowChanged, cca.TimerOff},
		{"testRecordVideoWithTimer", testRecordVideoWithTimer, cca.TimerOn},
		{"testRecordCancelTimer", testRecordCancelTimer, cca.TimerOn},
		{"testVideoSnapshot", testVideoSnapshot, cca.TimerOff},
		{"testStopInPause", testStopInPause, cca.TimerOff},
		{"testPauseResume", testPauseResume, cca.TimerOff},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, time.Second*5)
			defer cancel()

			app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb, false)
			if err != nil {
				s.Fatal("Failed to open CCA: ", err)
			}
			defer (func(ctx context.Context) {
				if err := app.Close(ctx); err != nil {
					s.Error("Failed to close app: ", err)
				}
			})(cleanupCtx)

			testing.ContextLog(ctx, "Switch to video mode")
			if err := app.SwitchMode(ctx, cca.Video); err != nil {
				s.Fatal("Failed to switch to video mode: ", err)
			}
			if err := app.WaitForVideoActive(ctx); err != nil {
				s.Fatal("Preview is inactive after switch to video mode: ", err)
			}

			if err := app.RunThroughCameras(ctx, func(facing cca.Facing) error {
				if err := app.SetTimerOption(ctx, tc.timer); err != nil {
					return errors.Wrapf(err, "failed to set timer option %v", tc.timer)
				}
				if err := tc.run(ctx, app); err != nil {
					return errors.Wrap(err, "failed in running test")
				}
				return nil
			}); err != nil {
				s.Fatal("Failed to run tests through all cameras: ", err)
			}
		})
	}
}

func testRecordVideoWithWindowChanged(ctx context.Context, app *cca.App) error {
	dirs, err := app.SavedDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get CCA default saved path")
	}
	testing.ContextLog(ctx, "Click on start shutter")
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Maximizing window")
	if err := app.MaximizeWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to maximize window")
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Restore window")
	if err := app.RestoreWindow(ctx); err != nil {
		return errors.Wrap(err, "failed to restore window")
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}
	start := time.Now()
	testing.ContextLog(ctx, "Click on stop shutter")
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "shutter is not ended")
	}
	if _, err := app.WaitForFileSaved(ctx, dirs, cca.VideoPattern, start); err != nil {
		return errors.Wrap(err, "cannot find result video")
	}
	return nil
}

func testRecordVideoWithTimer(ctx context.Context, app *cca.App) error {
	_, err := app.RecordVideo(ctx, cca.TimerOn, time.Second)
	return err
}

func testRecordCancelTimer(ctx context.Context, app *cca.App) error {
	testing.ContextLog(ctx, "Click on start shutter")
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}
	testing.ContextLog(ctx, "Click on cancel shutter")
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if taking, err := app.GetState(ctx, "taking"); err != nil {
		return err
	} else if taking {
		return errors.New("shutter is not cancelled after clicking cancel shutter")
	}
	return nil
}

func startRecording(ctx context.Context, app *cca.App) error {
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := app.WaitForState(ctx, "recording", true); err != nil {
		return errors.Wrap(err, "recording is not started")
	}
	return nil
}

func testVideoSnapshot(ctx context.Context, app *cca.App) error {
	startTime := time.Now()
	if err := startRecording(ctx, app); err != nil {
		return err
	}

	// Take a video snapshot.
	if err := app.Click(ctx, cca.VideoSnapshotButton); err != nil {
		return err
	}
	dirs, err := app.SavedDirs(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get saved directories")
	}
	if _, err := app.WaitForFileSaved(ctx, dirs, cca.PhotoPattern, startTime); err != nil {
		return errors.Wrap(err, "failed find saved video snapshot file")
	}

	if _, err := app.StopRecording(ctx, cca.TimerOff, startTime); err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	return nil
}

// startRecordAndPause starts recording for 1 second and pause the recording.
func startRecordAndPause(ctx context.Context, app *cca.App) error {
	if err := startRecording(ctx, app); err != nil {
		return err
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	if err := app.Click(ctx, cca.VideoPauseResumeButton); err != nil {
		return errors.Wrap(err, "failed to pause recording")
	}
	if err := app.WaitForState(ctx, "recording-paused", true); err != nil {
		return errors.Wrap(err, "failed to resume recording")
	}
	return nil
}

func videoDuration(ctx context.Context, app *cca.App, info os.FileInfo) (time.Duration, error) {
	path, err := app.FilePathInSavedDirs(ctx, info.Name())
	if err != nil {
		return 0, errors.Wrap(err, "failed to get file path in saved path")
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to open file %v", path)
	}
	defer f.Close()

	fraInfo, err := mp4.ProbeFra(f)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to probe fragments from %v", path)
	}

	duration := 0.0
	if len(fraInfo.Segments) == 0 {
		// Regular MP4
		boxes, err := mp4.ExtractBoxWithPayload(f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()})
		if err != nil {
			return 0, errors.Wrapf(err, "failed to parse mp4 header from %v", path)
		}
		if len(boxes) == 0 {
			return 0, errors.New("no mvhd box found")
		}
		mvhd, ok := boxes[0].Payload.(*mp4.Mvhd)
		if !ok {
			return 0, errors.New("got invalid mvhd box")
		}
		duration = float64(mvhd.DurationV0) / float64(mvhd.TimescaleV0)
		// TODO(crbug.com/1140852): Remove the logging once we fully migrated to regular mp4.
		testing.ContextLogf(ctx, "Found a regular mp4 with duration %.2fs", duration)
	} else {
		// Fragmented MP4
		// TODO(crbug.com/1140852): Remove fmp4 code path once we fully migrated to regular mp4.
		for _, s := range fraInfo.Segments {
			duration += float64(s.Duration) / float64(fraInfo.Tracks[s.TrackID-1].Timescale)
		}
		testing.ContextLogf(ctx, "Found a fragmented mp4 with duration %.2fs", duration)
	}

	return time.Duration(duration * float64(time.Second)), nil
}

// stopRecordWithDuration stops recording and checks the result video with expected duration.
func stopRecordWithDuration(ctx context.Context, app *cca.App, startTime time.Time, expected time.Duration) error {
	info, err := app.StopRecording(ctx, cca.TimerOff, startTime)
	if err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	duration, err := videoDuration(ctx, app, info)
	if err != nil {
		return errors.Wrap(err, "failed to get video duration")
	}

	if duration > expected+durationTolerance || duration < expected-durationTolerance {
		return errors.Errorf("incorrect result video duration get %v; want %v with tolerance %v",
			duration, expected, durationTolerance)
	}
	return nil
}

func testStopInPause(ctx context.Context, app *cca.App) error {
	startTime := time.Now()
	if err := startRecordAndPause(ctx, app); err != nil {
		return errors.Wrap(err, "failed to start and pause recording")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	return stopRecordWithDuration(ctx, app, startTime, time.Second)
}

func testPauseResume(ctx context.Context, app *cca.App) error {
	startTime := time.Now()
	if err := startRecordAndPause(ctx, app); err != nil {
		return errors.Wrap(err, "failed to start and pause recording")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	// Resume recording.
	if err := app.Click(ctx, cca.VideoPauseResumeButton); err != nil {
		return errors.Wrap(err, "failed to resume recording")
	}
	if err := app.WaitForState(ctx, "recording-paused", false); err != nil {
		return errors.Wrap(err, "failed to wait for resume recording state")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	return stopRecordWithDuration(ctx, app, startTime, 2*time.Second)
}
