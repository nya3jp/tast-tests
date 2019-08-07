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
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUITakePicture(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)
	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
	app.RemoveCacheData(ctx,
		[]string{"toggle3sec", "toggle10sec", "toggle3x3", "toggle4x4", "toggleGolden"})

	restartApp := func() {
		if err := app.Restart(ctx); err != nil {
			s.Fatal("Failed to restart CCA: ", err)
		}
		if err := app.WaitForVideoActive(ctx); err != nil {
			s.Fatal("Preview is inactive after restart App: ", err)
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
