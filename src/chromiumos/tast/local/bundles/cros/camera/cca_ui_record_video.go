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
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIRecordVideo(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	testRecordVideo := func() func(context.Context, *cca.App) error {
		return func(ctx context.Context, app *cca.App) error {
			dir, err := app.SavedDir(ctx)
			if err != nil {
				s.Fatal("Failed to get CCA default saved path: ", err)
			}
			return testRecordVideoWithWindowChanged(ctx, app, dir)
		}
	}

	for _, tst := range []struct {
		name  string
		run   func(context.Context, *cca.App) error
		timer cca.TimerState
	}{
		{"testRecordVideo", testRecordVideo(), cca.TimerOff},
		{"testRecordVideoWithTimer", testRecordVideoWithTimer, cca.TimerOn},
		{"testRecordCancelTimer", testRecordCancelTimer, cca.TimerOn},
		{"testVideSnapshot", testVideSnapshot, cca.TimerOff},
	} {
		s.Run(ctx, tst.name, func(ctx context.Context, s *testing.State) {
			app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
			if err != nil {
				s.Fatal("Failed to open CCA: ", err)
			}
			defer app.Close(ctx)
			defer (func() {
				if err := app.CheckJSError(ctx, s.OutDir()); err != nil {
					s.Error("Failed with javascript errors: ", err)
				}
			})()

			testing.ContextLog(ctx, "Switch to video mode")
			if err := app.SwitchMode(ctx, cca.Video); err != nil {
				s.Fatal("Failed to switch to video mode: ", err)
			}
			if err := app.WaitForVideoActive(ctx); err != nil {
				s.Fatal("Preview is inactive after switch to video mode: ", err)
			}

			if err := app.RunThroughCameras(ctx, func(facing cca.Facing) error {
				if err := app.SetTimerOption(ctx, tst.timer); err != nil {
					s.Fatalf("Failed to set timer option %v: %v", tst.timer, err)
				}
				if err := tst.run(ctx, app); err != nil {
					s.Error("Test failed: ", err)
				}
				return nil
			}); err != nil {
				s.Fatal("Failed to run tests through all cameras: ", err)
			}
		})
	}
}

func testRecordVideoWithWindowChanged(ctx context.Context, app *cca.App, dir string) error {
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

func testVideSnapshot(ctx context.Context, app *cca.App) error {
	// Start recording.
	startTime := time.Now()
	if err := app.ClickShutter(ctx); err != nil {
		return err
	}
	if err := app.WaitForState(ctx, "recording", true); err != nil {
		return errors.Wrap(err, "recording is not started")
	}

	// Take a video snapshot.
	if err := app.Click(ctx, cca.VideoSnapshotButton); err != nil {
		return err
	}
	dir, err := app.SavedDir(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get saved directory")
	}
	if _, err := app.WaitForFileSaved(ctx, dir, cca.PhotoPattern, startTime); err != nil {
		return errors.Wrap(err, "failed find saved video snapshot file")
	}

	if _, err := app.StopRecording(ctx, cca.TimerOff, startTime); err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	return nil
}
