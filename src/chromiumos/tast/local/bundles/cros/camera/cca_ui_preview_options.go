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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIPreviewOptions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
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

	checkMirror := func() bool {
		var facing cca.Facing
		if facing, err = app.GetFacing(ctx); err != nil {
			s.Fatal("Failed to get camera facing")
			return false
		}
		// Mirror should be enabled for front / external camera and should be
		// disabled for back camera.
		var mirrored bool
		if mirrored, err = app.Mirrored(ctx); err != nil {
			s.Error("Failed to get mirrored state: ", err)
		} else if mirrored != (facing != cca.FacingBack) {
			s.Errorf("Mirroring state is unexpected: got %v, want %v", mirrored, facing != cca.FacingBack)
		}
		return mirrored
	}

	// Check mirror for default camera.
	firstCameraDefaultMirror := checkMirror()

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
		for i := 1; i < numCameras; i++ {
			// Switch camera.
			if err := app.SwitchCamera(ctx); err != nil {
				s.Fatal("Switching camera failed: ", err)
			}

			// Check default mirrored.
			checkMirror()
		}

		// Switch back to the first camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switching camera failed: ", err)
		}

		// Mirror state should persist for each camera respectively. Since the
		// mirror state of first camera is toggled, the state should be different
		// from the default one.
		if mirrored, err := app.Mirrored(ctx); err != nil {
			s.Error("Failed to get mirrored state: ", err)
		} else if mirrored == firstCameraDefaultMirror {
			s.Error("Mirroring does not persist correctly")
		}
	}
}
