// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIExpert,
		Desc:         "Opens CCA and verifies the expert options",
		Contacts:     []string{"kaihsien@google.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIExpert(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	}
	s.Log("Preview started")

	// Check that changing expert mode activation in settings is effective.
	if enabled, err := app.ToggleExpertMode(ctx); err != nil {
		s.Error("Failed to enabled expert mode: ", err)
	} else if !enabled {
		s.Error("Expert mode is not enabled")
	}

	if err := app.ClickWithSelector(ctx, "#expert-show-metadata"); err != nil {
		s.Error("Failed to click show metadata button: ", err)
	}
	if err := app.ClickWithSelector(ctx, "#expert-save-metadata"); err != nil {
		s.Error("Failed to click save metadata button: ", err)
	}
	testing.Sleep(ctx, time.Second)

	testExpert := func() {
		if working, err := app.MetadataVisible(ctx); err != nil {
			s.Error("Failed to check show metadata status: ", err)
		} else if !working {
			s.Error("Metadata is not showing correctly")
		}

		if _, err := app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Error("Failed when taking photo: ", err)
		}
	}
	testExpert()

	if err := app.SwitchMode(ctx, cca.Square); err != nil {
		s.Fatal("Failed to switch to square mode: ", err)
	} else {
		testExpert()
	}

	if err := app.SwitchCamera(ctx); err != nil {
		s.Fatal("Switch camera failed: ", err)
	} else {
		testExpert()
	}

	if err := app.Restart(ctx); err != nil {
		s.Fatal("Failed to restart CCA: ", err)
	} else if err = app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	} else {
		testExpert()
	}
}
