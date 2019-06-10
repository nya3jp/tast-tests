// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
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
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUITakePicture(ctx context.Context, s *testing.State) {

	launchApp := func(cr *chrome.Chrome, app *cca.App, restartChrome bool) (*chrome.Chrome, *cca.App) {
		var err error
		if app != nil {
			app.Close(ctx)
		}
		if restartChrome {
			cr.Close(ctx)
			if cr, err = chrome.New(ctx); err != nil {
				s.Fatal("Failed restart chrome: ", err)
			}
		}
		if app, err = cca.New(ctx, cr, []string{
			s.DataPath("cca_ui.js"),
			s.DataPath("cca_ui_capture.js")}); err != nil {
			s.Fatal("Failed to open CCA: ", err)
		}
		return cr, app
	}

	cr, app := launchApp(s.PreValue().(*chrome.Chrome), nil, false)
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
	s.Log("Preview started")

	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		s.Fatal("Failed to switch to photo mode: ", err)
	}

	if err := testTakeSinglePhoto(ctx, s, app); err != nil {
		s.Error("Failed in testTakeSinglePhoto(): ", err)
		cr, app = launchApp(cr, app, true)
	}

	if err := testTakeSinglePhotoWithTimer(ctx, s, app); err != nil {
		s.Error("Failed in testTakeSinglePhoto(): ", err)
		cr, app = launchApp(cr, app, true)
	}

	if err := testCancelTimer(ctx, s, app); err != nil {
		s.Error("Failed in testTakeSinglePhoto(): ", err)
		cr, app = launchApp(cr, app, true)
	}
}

func ensureTimerOption(ctx context.Context, s *testing.State, app *cca.App, active bool) {
	if cur, err := app.GetOption(ctx, "timer"); err != nil {
		s.Fatal("Failed to get timer state: ", err)
	} else if cur != active {
		if cur, err = app.ToggleTimerOption(ctx); err != nil {
			s.Fatal("Failed to toggle timer state: ", err)
		}
		if cur != active {
			s.Fatal("Timer state does not changed after toggle")
		}
	}
}

func clickShutter(ctx context.Context, s *testing.State, app *cca.App) {
	if err := app.ClickShutter(ctx); err != nil {
		s.Fatal("Failed to click shutter button: ", err)
	}
}

func testTakeSinglePhoto(ctx context.Context, s *testing.State, app *cca.App) error {
	ensureTimerOption(ctx, s, app, false)
	clickShutter(ctx, s, app)
	if _, err := app.WaitForSavedFile(ctx, cca.PhotoPattern, time.Now()); err != nil {
		return errors.Wrap(err, "Cannot find result picture")
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "Shutter is not ended")
	}
	return errors.New("Intended Test error")
}

func testTakeSinglePhotoWithTimer(ctx context.Context, s *testing.State, app *cca.App) error {
	ensureTimerOption(ctx, s, app, true)
	delay := 3 * time.Second
	if delay10, err := app.GetOption(ctx, "_10sec"); err != nil {
		s.Fatal("Failed to get 10 second timer state: ", err)
	} else if delay10 {
		delay = 10 * time.Second
	}
	now := time.Now()
	clickShutter(ctx, s, app)

	if image, err := app.WaitForSavedFile(ctx, cca.PhotoPattern, now); err != nil {
		return errors.Wrap(err, "Cannot find result picture")
	} else if elapsed := image.ModTime().Sub(now); elapsed < delay {
		return errors.Errorf("The capture should happen after timer of %v nm, actual elapsed time %v nm", delay, elapsed)
	}
	if err := app.WaitForState(ctx, "taking", false); err != nil {
		return errors.Wrap(err, "Shutter is not ended")
	}
	return nil
}

func testCancelTimer(ctx context.Context, s *testing.State, app *cca.App) error {
	ensureTimerOption(ctx, s, app, true)
	delay := 3 * time.Second
	if delay10, err := app.GetOption(ctx, "_10sec"); err != nil {
		s.Fatal("Failed to get 10 second timer state: ", err)
	} else if delay10 {
		delay = 10 * time.Second
	}

	testing.ContextLog(ctx, "Click on start shutter")
	now := time.Now()
	elapsed := delay - time.Second
	clickShutter(ctx, s, app)
	if err := testing.Sleep(ctx, elapsed); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Click on cancel shutter")
	clickShutter(ctx, s, app)
	path, err := app.GetSavedDir(ctx)
	if err != nil {
		return err
	}

	if err := testing.Sleep(ctx, 3*time.Second); err != nil {
		return err
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrap(err, "failed to read the directory for saving media files")
	}
	for _, file := range files {
		if file.ModTime().After(now) && cca.PhotoPattern.MatchString(file.Name()) {
			return errors.Errorf("Captured file %v found after shutter canceled", file.Name())
		}
	}
	return nil
}
