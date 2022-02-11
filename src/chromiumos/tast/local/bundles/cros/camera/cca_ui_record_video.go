// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"os"
	"time"

	"github.com/abema/go-mp4"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIRecordVideo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Opens CCA and verifies video recording related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Timeout:      5 * time.Minute,
		Fixture:      "ccaTestBridgeReady",
		Params: []testing.Param{{
			Val: false,
		}, {
			Name: "multi_stream",
			Val:  true,
		}},
	})
}

// durationTolerance is the tolerate difference when comparing video duration.
const durationTolerance = 300 * time.Millisecond

type videoState string

const (
	// videoStateEmpty is the state before video start recording.
	videoStateEmpty videoState = "empty"
	// videoStateRecording is the state when video is recording.
	videoStateRecording = "recording"
	// videoStatePaused is the state when video is paused.
	videoStatePaused = "paused"
	// videoStateStopped is the state when video is finished recording.
	videoStateStopped = "stopped"
)

func checkState(actual videoState, expecteds ...videoState) error {
	found := false
	for _, expected := range expecteds {
		if actual == expected {
			found = true
			break
		}
	}
	if !found {
		return errors.Errorf("Assert video in %v state(s); get %v state", expecteds, actual)
	}
	return nil
}

// video tracks the expected video state across multiple CCA video operations.
type video struct {
	// state is the current recording state of video.
	state videoState
	// last is the last timing of toggling |state|.
	last time.Time
	// d aggregates expected duration of all finished video clips.
	d time.Duration
}

func newVideo() *video {
	return &video{state: videoStateEmpty}
}

func (v *video) start(ctx context.Context, app *cca.App) error {
	if err := checkState(v.state, videoStateEmpty); err != nil {
		return err
	}
	t, err := app.TriggerStateChange(ctx, "recording", true, func() error {
		return app.ClickShutter(ctx)
	})
	if err != nil {
		return err
	}
	v.state = videoStateRecording
	v.last = t
	v.d = time.Duration(0)
	return nil
}

func (v *video) pause(ctx context.Context, app *cca.App) error {
	if err := checkState(v.state, videoStateRecording); err != nil {
		return err
	}
	t, err := app.TriggerStateChange(ctx, "recording-paused", true, func() error {
		if err := app.Click(ctx, cca.VideoPauseResumeButton); err != nil {
			return errors.Wrap(err, "failed to pause recording")
		}
		return nil
	})
	if err != nil {
		return err
	}
	v.d += t.Sub(v.last)
	v.state = videoStatePaused
	v.last = t
	return nil
}

func (v *video) resume(ctx context.Context, app *cca.App) error {
	if err := checkState(v.state, videoStatePaused); err != nil {
		return err
	}
	t, err := app.TriggerStateChange(ctx, "recording-paused", false, func() error {
		if err := app.Click(ctx, cca.VideoPauseResumeButton); err != nil {
			return errors.Wrap(err, "failed to resume recording")
		}
		return nil
	})
	if err != nil {
		return err
	}
	v.state = videoStateRecording
	v.last = t
	return nil
}

func (v *video) stop(ctx context.Context, app *cca.App) error {
	if err := checkState(v.state, videoStateRecording, videoStatePaused); err != nil {
		return err
	}
	info, t, err := app.StopRecording(ctx, cca.TimerOff, v.last)
	if err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	if v.state == videoStateRecording {
		v.d += t.Sub(v.last)
	}
	v.state = videoStateStopped
	v.last = t

	// Check duration from result video file with expected duration.
	path, err := app.FilePathInSavedDir(ctx, info.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get file path in saved path")
	}
	duration, err := videoDuration(ctx, path)
	if err != nil {
		return err
	}
	if duration > v.d+durationTolerance || duration < v.d-durationTolerance {
		return errors.Errorf("incorrect result video duration get %v; want %v with tolerance %v", duration, v.d, durationTolerance)
	}
	return nil
}

func CCAUIRecordVideo(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(cca.FixtureData).Chrome
	runTestWithApp := s.FixtValue().(cca.FixtureData).RunTestWithApp
	enableMultiStream := s.Param().(bool)
	subTestTimeout := 40 * time.Second
	for _, tc := range []struct {
		name  string
		run   func(context.Context, *cca.App) error
		timer cca.TimerState
	}{
		{"testRecordVideoWithWindowChanged", testRecordVideoWithWindowChanged, cca.TimerOff},
		{"testVideoProfile", testVideoProfile, cca.TimerOff},
		{"testRecordVideoWithTimer", testRecordVideoWithTimer, cca.TimerOn},
		{"testRecordCancelTimer", testRecordCancelTimer, cca.TimerOn},
		{"testVideoSnapshot", testVideoSnapshot, cca.TimerOff},
		{"testStopInPause", testStopInPause, cca.TimerOff},
		{"testPauseResume", testPauseResume, cca.TimerOff},
	} {
		subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
		s.Run(subTestCtx, tc.name, func(ctx context.Context, s *testing.State) {
			if err := runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
				cleanupCtx := ctx
				ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
				defer cancel()

				if enableMultiStream {
					if err := app.SetEnableMultiStreamRecording(ctx, true); err != nil {
						return errors.Wrap(err, "failed to enable multi-stream recording")
					}
					defer app.SetEnableMultiStreamRecording(cleanupCtx, false)
				}
				testing.ContextLog(ctx, "Switch to video mode")
				if err := app.SwitchMode(ctx, cca.Video); err != nil {
					return errors.Wrap(err, "failed to switch to video mode")
				}
				if err := app.WaitForVideoActive(ctx); err != nil {
					return errors.Wrap(err, "preview is inactive after switch to video mode")
				}
				return app.RunThroughCameras(ctx, func(_ cca.Facing) error {
					if err := app.SetTimerOption(ctx, tc.timer); err != nil {
						return errors.Wrapf(err, "failed to set timer option %v", tc.timer)
					}
					return tc.run(ctx, app)
				})
			}, cca.TestWithAppParams{}); err != nil {
				s.Errorf("Failed to pass %v subtest: %v", tc.name, err)
			}
		})
		cancel()
	}

	subTestCtx, cancel := context.WithTimeout(ctx, subTestTimeout)
	s.Run(subTestCtx, "testConfirmDialog", func(ctx context.Context, s *testing.State) {
		if err := runTestWithApp(ctx, func(ctx context.Context, app *cca.App) error {
			return testConfirmDialog(ctx, app, cr)
		}, cca.TestWithAppParams{StopAppOnlyIfExist: true}); err != nil {
			s.Error("Failed to pass confirm dialog subtest: ", err)
		}
	})
	cancel()
}

func testRecordVideoWithWindowChanged(ctx context.Context, app *cca.App) error {
	dir, err := app.SavedDir(ctx)
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
	if _, err := app.WaitForFileSaved(ctx, dir, cca.VideoPattern, start); err != nil {
		return errors.Wrap(err, "cannot find result video")
	}
	return nil
}

func testVideoProfile(ctx context.Context, app *cca.App) error {
	file, err := app.RecordVideo(ctx, cca.TimerOn, time.Second)

	path, err := app.FilePathInSavedDir(ctx, file.Name())
	if err != nil {
		return err
	}

	return cca.CheckVideoProfile(path, cca.ProfileH264High)
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
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return err
	}
	return nil
}

func testVideoSnapshot(ctx context.Context, app *cca.App) error {
	testing.ContextLog(ctx, "Click on start shutter")
	startTime := time.Now()
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := app.WaitForState(ctx, "recording", true); err != nil {
		return errors.Wrap(err, "recording is not started")
	}

	// Ensure video have at least 1s duration.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep in video duration")
	}

	// Take a video snapshot.
	if err := app.Click(ctx, cca.VideoSnapshotButton); err != nil {
		return err
	}
	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get saved directories")
	}
	if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, startTime); err != nil {
		return errors.Wrap(err, "failed find saved video snapshot file")
	}

	if _, _, err := app.StopRecording(ctx, cca.TimerOff, startTime); err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	return nil
}

// startRecordAndPause starts recording for 1 second and pauses the recording.
func startRecordAndPause(ctx context.Context, app *cca.App) (*video, error) {
	v := newVideo()
	if err := v.start(ctx, app); err != nil {
		return nil, err
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return nil, err
	}
	if err := v.pause(ctx, app); err != nil {
		return nil, err
	}
	return v, nil
}

func videoDuration(ctx context.Context, path string) (time.Duration, error) {
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
		duration = float64(mvhd.DurationV0) / float64(mvhd.Timescale)
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

func testStopInPause(ctx context.Context, app *cca.App) error {
	v, err := startRecordAndPause(ctx, app)
	if err != nil {
		return errors.Wrap(err, "failed to start and pause recording")
	}

	if err := testing.Sleep(ctx, time.Second); err != nil {
		return err
	}

	return v.stop(ctx, app)
}

func testPauseResume(ctx context.Context, app *cca.App) error {
	v, err := startRecordAndPause(ctx, app)
	if err != nil {
		return errors.Wrap(err, "failed to start and pause recording")
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep 1 second in pausing state")
	}
	if err := v.resume(ctx, app); err != nil {
		return err
	}
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep 1 second after resuming")
	}

	return v.stop(ctx, app)
}

func testConfirmDialog(ctx context.Context, app *cca.App, cr *chrome.Chrome) error {
	if err := app.TriggerConfiguration(ctx, func() error {
		testing.ContextLog(ctx, "Switch to video mode")
		if err := app.SwitchMode(ctx, cca.Video); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Start Recording")
	startTime := time.Now()
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := app.WaitForState(ctx, "recording", true); err != nil {
		return errors.Wrap(err, "recording is not started")
	}

	testing.ContextLog(ctx, "Try to close camera app")
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		return err
	}
	defer keyboard.Close()

	// Try to close the camera app.
	if err := keyboard.Accel(ctx, "ctrl+W"); err != nil {
		return err
	}

	// It is expected that the camera app is not closed.
	errTimeout := errors.New("CCA exists after timeout")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		appExist, err := cca.InstanceExists(ctx, cr)
		if err != nil {
			return testing.PollBreak(err)
		}
		if !appExist {
			return testing.PollBreak(errors.New("CCA is unexpectedly closed"))
		}
		return errTimeout
	}, &testing.PollOptions{Timeout: 3 * time.Second}); !errors.Is(err, errTimeout) {
		return err
	}

	// Dismiss the confirm dialog.
	if err := keyboard.Accel(ctx, "esc"); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Stop recording")
	if _, _, err := app.StopRecording(ctx, cca.TimerOff, startTime); err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}

	testing.ContextLog(ctx, "Try to close camera app")
	// Try to close the camera app again.
	if err := keyboard.Accel(ctx, "ctrl+W"); err != nil {
		return err
	}

	// Now the camera app is closable.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		appExist, err := cca.InstanceExists(ctx, cr)
		if err != nil {
			return testing.PollBreak(err)
		}
		if appExist {
			return errors.New("CCA is not closed")
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		return err
	}
	return nil
}
