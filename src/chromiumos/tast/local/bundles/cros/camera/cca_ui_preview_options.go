// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	// TODO(crbug.com/963772): Move libraries in video to camera or media folder.
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIPreviewOptions,
		Desc:         "Opens CCA and verifies the use cases of preview options like grid and mirror",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js", "cca_ui_preview_options.js", "cca_ui_multi_camera.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIPreviewOptions(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js"),
		s.DataPath("cca_ui_preview_options.js"), s.DataPath("cca_ui_multi_camera.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.CheckVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching App: ", err)
	}
	s.Log("Preview started")

	if err := app.CheckMirrorButtonExist(ctx, true); err != nil {
		s.Fatal("Check mirror button existence failed: ", err)
	}

	// The default camera should be front camera, and mirror should be enabled.
	if err := app.CheckMirror(ctx, true); err != nil {
		s.Fatal("Check mirror state failed: ", err)
	}

	err = app.ToggleMirrorOption(ctx)
	if err != nil {
		s.Fatal("Toggle mirror option failed: ", err)
	}

	if err := app.CheckMirror(ctx, false); err != nil {
		s.Fatal("Check mirror state failed: ", err)
	}

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}

	if numCameras > 1 {
		s.Log("Test multi-camera scenario")
		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}

		facing, err := app.GetFacing(ctx)
		if err != nil {
			s.Fatal("Get facing failed: ", err)
		}

		// Front facing and external camera should turn on mirror by default.
		// Back camera should not.
		if err := app.CheckMirror(ctx, facing != cca.FacingBack); err != nil {
			s.Fatal("Check mirror state failed: ", err)
		}

		// Switch camera.
		if err := app.SwitchCamera(ctx); err != nil {
			s.Fatal("Switch camera failed: ", err)
		}

		// Mirror state should persist for each camera respectively.
		if err := app.CheckMirror(ctx, false); err != nil {
			s.Fatal("Check mirror state failed: ", err)
		}
	}
}
