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
		Func:         CCAUITakePicture,
		Desc:         "Opens CCA and verifies photo taking related use cases",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUITakePicture(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

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

	restartApp := func() {
		s.Log("Restarts CCA")
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
	}

	for _, tst := range []struct {
		name     string
		testFunc func(context.Context, *cca.App) error
	}{
		{"testTakeSinglePhoto", testTakeSinglePhoto},
		{"testTakeSinglePhotoWithTimer", testTakeSinglePhotoWithTimer},
		{"testCancelTimer", testCancelTimer},
	} {
		if err := tst.testFunc(ctx, app); err != nil {
			s.Errorf("Failed in %v(): %v", tst.name, err)
			restartApp()
		}
	}
}

func testTakeSinglePhoto(ctx context.Context, app *cca.App) error {
	_, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	return err
}

func testTakeSinglePhotoWithTimer(ctx context.Context, app *cca.App) error {
	_, err := app.TakeSinglePhoto(ctx, cca.TimerOn)
	return err
}

func testCancelTimer(ctx context.Context, app *cca.App) error {
	if err := app.SetTimerOption(ctx, true); err != nil {
		return err
	}

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
