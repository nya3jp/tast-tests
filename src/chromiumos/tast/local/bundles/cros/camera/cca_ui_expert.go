// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIExpert,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Opens CCA and verifies the expert options",
		Contacts:     []string{"inker@chromium.org", "shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", "arc_camera3", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIExpert(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()
	for i, action := range []struct {
		Name    string
		Func    func(context.Context, *cca.App) error
		Enabled bool
	}{
		// Expert mode is not reset after each test for persistency
		{"toggleExpertMode", toggleExpertMode, false},
		{"toggleExpertModeOptions", toggleExpertModeOptions, true},
		{"switchModeAndBack", switchModeAndBack, true},
		{"toggleExpertMode", toggleExpertMode, false},
		{"toggleExpertMode", toggleExpertMode, true},
		{"toggleExpertModeOptions", toggleExpertModeOptions, false},
		{"disableExpertModeOnUI", disableExpertModeOnUI, false},
		{"enableExpertModeOnUI", enableExpertModeOnUI, false},
	} {
		if err := action.Func(ctx, app); err != nil {
			s.Fatalf("Failed to perform action %v of test %v: %v", action.Name, i, err)
		}
		if err := verifyExpertMode(ctx, app, action.Enabled); err != nil {
			s.Fatalf("Failed in test %v %v(): %v", i, action.Name, err)
		}
	}
}

func verifyExpertMode(ctx context.Context, app *cca.App, enabled bool) error {
	if err := app.CheckMetadataVisibility(ctx, enabled); err != nil {
		return err
	}
	if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
		return err
	}
	return nil
}

func toggleExpertMode(ctx context.Context, app *cca.App) error {
	_, err := app.ToggleExpertMode(ctx)
	// TODO(crbug.com/1039991): There are asynchronous mojo IPC calls happens
	// after toggling, and we don't have a way to poll it properly without
	// significantly refactor the logic.
	testing.Sleep(ctx, time.Second)
	return err
}

func toggleExpertModeOptions(ctx context.Context, app *cca.App) error {
	if err := cca.MainMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.MainMenu.Close(ctx, app)

	if err := cca.ExpertMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.ExpertMenu.Close(ctx, app)

	if _, err := app.ToggleOption(ctx, cca.ShowMetadataOption); err != nil {
		return err
	}
	if _, err := app.ToggleOption(ctx, cca.SaveMetadataOption); err != nil {
		return err
	}
	return nil
}

func switchModeAndBack(ctx context.Context, app *cca.App) error {
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		return errors.Wrap(err, "failed to switch back to photo mode")
	}
	return nil
}

func disableExpertModeOnUI(ctx context.Context, app *cca.App) error {
	if err := cca.MainMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.MainMenu.Close(ctx, app)

	if err := cca.ExpertMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.ExpertMenu.Close(ctx, app)

	if _, err := app.ToggleOption(ctx, cca.ExpertModeOption); err != nil {
		return err
	}
	return nil
}

func enableExpertModeOnUI(ctx context.Context, app *cca.App) error {
	if err := cca.MainMenu.Open(ctx, app); err != nil {
		return err
	}
	defer cca.MainMenu.Close(ctx, app)

	// Clicking setting header 5 times should enable expert mode. (b/190696285)
	for i := 0; i < 5; i++ {
		if err := app.ClickWithSelector(ctx, "#settings-header"); err != nil {
			return err
		}
	}

	if err := app.WaitForState(ctx, "expert", true); err != nil {
		return err
	}

	return nil
}
