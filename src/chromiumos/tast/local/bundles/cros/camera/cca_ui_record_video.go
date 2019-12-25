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

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)
	defer app.RemoveCacheData(ctx, []string{"toggleTimer"})
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}

	testing.ContextLog(ctx, "Switch to video mode")
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Fatal("Failed to switch to video mode: ", err)
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after switch to video mode: ", err)
	}

	toggleTimer := func(timerState cca.TimerState) func(context.Context, *cca.App) error {
		return func(ctx context.Context, app *cca.App) error {
			return app.SetTimerOption(ctx, timerState)
		}
	}

	dir, err := cca.GetSavedDir(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get CCA default saved path: ", err)
	}

	testRecordVideo := func() func(context.Context, *cca.App) error {
		return func(ctx context.Context, app *cca.App) error {
			return testRecordVideoWithWindowChanged(ctx, app, dir)
		}
	}

	if err := app.RunThroughCameras(ctx, func(facing cca.Facing) error {
		for _, action := range []struct {
			name string
			run  func(context.Context, *cca.App) error
		}{
			{"testRecordVideo", testRecordVideo()},
			{"toggleTimer(cca.TimerOn)", toggleTimer(cca.TimerOn)},
			{"testRecordVideoWithTimer", testRecordVideoWithTimer},
			{"testRecordCancelTimer", testRecordCancelTimer},
			{"toggleTimer(cca.TimerOff)", toggleTimer(cca.TimerOff)},
		} {
			testing.ContextLog(ctx, "Start ", action.name)
			if err := action.run(ctx, app); err != nil {
				return errors.Wrapf(err, "failed in %v()", action.name)
			}
			testing.ContextLog(ctx, "Finish ", action.name)
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to run tests through all cameras: ", err)
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
