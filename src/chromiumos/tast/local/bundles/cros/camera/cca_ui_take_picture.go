// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUITakePicture,
		Desc:         "Opens CCA and verifies photo taking related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js", "cca_ui_capture.js"},
	})
}

func CCAUITakePicture(ctx context.Context, s *testing.State) {
	launchApp := func() (*chrome.Chrome, *cca.App) {
		cr, err := chrome.New(ctx)
		if err != nil {
			s.Fatal("Failed to start chrome: ", err)
		}
		app, err := cca.New(ctx, cr, []string{
			s.DataPath("cca_ui.js"),
			s.DataPath("cca_ui_capture.js")})
		if err != nil {
			s.Fatal("Failed to open CCA: ", err)
		}
		if err := app.WaitForVideoActive(ctx); err != nil {
			s.Fatal("Preview is inactive after launching App: ", err)
		}
		s.Log("Preview started")
		if err := app.SwitchMode(ctx, cca.Photo); err != nil {
			s.Fatal("Failed to switch to photo mode: ", err)
		}
		return cr, app
	}
	closeApp := func(cr *chrome.Chrome, app *cca.App) {
		if err := app.Close(ctx); err != nil {
			s.Fatal("Failed to close app: ", err)
		}
		if err := cr.Close(ctx); err != nil {
			s.Fatal("Failed to close chrome: ", err)
		}
	}

	cr, app := launchApp()
	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *testing.State, *cca.App) error
	}{
		{"testTakeSinglePhoto", testTakeSinglePhoto},
		{"testTakeSinglePhotoWithTimer", testTakeSinglePhotoWithTimer},
		{"testCancelTimer", testCancelTimer},
	} {
		if err := tst.testFunc(ctx, s, app); err != nil {
			s.Errorf("Failed in %v(): %v", tst.name, err)
			closeApp(cr, app)
			cr, app = launchApp()
		}
	}
	closeApp(cr, app)
}

func ensureTimerOption(ctx context.Context, s *testing.State, app *cca.App, active bool) {
	if cur, err := app.GetState(ctx, "timer"); err != nil {
		s.Fatal("Failed to get timer state: ", err)
	} else if cur != active {
		if _, err := app.ToggleTimerOption(ctx); err != nil {
			s.Fatal("Failed to toggle timer state: ", err)
		}
	}
}

func clickShutter(ctx context.Context, s *testing.State, app *cca.App) {
	if err := app.ClickShutter(ctx); err != nil {
		s.Fatal("Failed to click shutter button: ", err)
	}
}

func getTimerDelay(ctx context.Context, s *testing.State, app *cca.App) time.Duration {
	delay, err := app.GetTimerDelay(ctx)
	if err != nil {
		s.Fatal("Failed to get timer delay: ", err)
	}
	return delay
}

func testTakeSinglePhoto(ctx context.Context, s *testing.State, app *cca.App) error {
	ensureTimerOption(ctx, s, app, false)
	now := time.Now()

	testing.ContextLog(ctx, "Click on start shutter")
	clickShutter(ctx, s, app)
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "Capturing hasn't ended")
	}
	if _, err := app.WaitForFileSaved(ctx, cca.PhotoPattern, now); err != nil {
		return errors.Wrap(err, "Cannot find result picture")
	}
	return nil
}

func testTakeSinglePhotoWithTimer(ctx context.Context, s *testing.State, app *cca.App) error {
	ensureTimerOption(ctx, s, app, true)
	delay := getTimerDelay(ctx, s, app)
	now := time.Now()

	testing.ContextLog(ctx, "Click on start shutter")
	clickShutter(ctx, s, app)
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "Shutter is not ended")
	}
	if image, err := app.WaitForFileSaved(ctx, cca.PhotoPattern, now); err != nil {
		return errors.Wrap(err, "Cannot find result picture")
	} else if elapsed := image.ModTime().Sub(now); elapsed < delay {
		return errors.Errorf("The capture should happen after timer of %v ns, actual elapsed time %v ns", delay, elapsed)
	}
	return nil
}

func testCancelTimer(ctx context.Context, s *testing.State, app *cca.App) error {
	ensureTimerOption(ctx, s, app, true)
	delay := getTimerDelay(ctx, s, app)

	testing.ContextLog(ctx, "Click on start shutter")
	now := time.Now()
	elapsed := delay - time.Second
	clickShutter(ctx, s, app)
	if err := testing.Sleep(ctx, elapsed); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Click on cancel shutter")
	clickShutter(ctx, s, app)
	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return err
	}
	if err := app.VerifyNoFileSaved(ctx, cca.PhotoPattern, now); err != nil {
		return errors.Wrap(err, "Captured file found after shutter canceled")
	}
	return nil
}
