// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreviewOptions,
		Desc:         "Opens CCA and verifies the use cases of preview options like grid and mirror",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIPreviewOptions(ctx context.Context, s *testing.State) {
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

	if exist, err := app.MirrorButtonExists(ctx); err != nil {
		s.Error("Failed to get mirroring button state: ", err)
	} else if !exist {
		s.Error("Mirroring button unexpectedly disappeared")
	}

	// The default camera should be front camera, and mirroring should be enabled.
	if mirrored, err := app.Mirrored(ctx); err != nil {
		s.Error("Failed to get mirrored state: ", err)
	} else if !mirrored {
		s.Error("Mirroring unexpectedly disabled")
	}

	_, err = app.ToggleMirroringOption(ctx)
	if err != nil {
		s.Fatal("Toggling mirror option failed: ", err)
	}

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	if numCameras > 1 {
		s.Log("Testing multi-camera scenario")
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switching camera failed: ", err)
		}

		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Geting facing failed: ", err)
		}

		// Front facing and external camera should turn on mirror by default.
		// Back camera should not.
		if mirrored, err := app.Mirrored(ctx); err != nil {
			s.Error("Failed to get mirrored state: ", err)
		} else if mirrored != (facing != cca.FacingBack) {
			s.Errorf("Mirroring state is unexpected: got %v, want %v", mirrored, facing != cca.FacingBack)
		}

		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switching camera failed: ", err)
		}

		// Mirror state should persist for each camera respectively.
		if mirrored, err := app.Mirrored(ctx); err != nil {
			s.Error("Failed to get mirrored state: ", err)
		} else if mirrored {
			s.Error("Mirroring unexpectedly enabled")
		}
	}
}
