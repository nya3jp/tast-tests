// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"image/jpeg"
	"os"
	"time"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	// TODO(crbug.com/963772): Move libraries in video to camera or media folder.
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIModes,
		Desc:         "Opens CCA and verifies the use cases of mode selector and portrait, square modes",
		Contacts:     []string{"shenghao@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js", "cca_ui_preview_options.js", "cca_ui_multi_camera.js", "human_face.y4m"},
	})
}

func CCAUIModes(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs(
		"--use-fake-ui-for-media-stream",
		"--use-fake-device-for-media-stream",
		"--use-file-for-fake-video-capture=" + s.DataPath("human_face.y4m")))
	if err != nil {
		s.Fatal("Failed to open chrome: ", err)
	}
	defer cr.Close(ctx)

	time.Sleep(time.Second * 5)

	app, err := cca.New(ctx, cr, []string{
		s.DataPath("cca_ui.js"),
		s.DataPath("cca_ui_preview_options.js"),
		s.DataPath("cca_ui_multi_camera.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	}
	s.Log("Preview started")




	// Switch to square mode and take photo.
	if err := app.SwitchMode(ctx, cca.Square); err != nil {
		s.Fatal("Failed to switch to square mode: ", err)
	}
	var fileInfos []os.FileInfo
	if fileInfos, err = app.TakeSinglePhoto(ctx, false); err != nil {
		s.Error("Failed to take square photo: ", err)
	}

	isSquarePhoto := func(info os.FileInfo, ctx context.Context) (bool, error) {
		file, err := os.Open(info.Name())
		if err != nil {
			return false, err
		}
		image, err := jpeg.Decode(file)
		if err != nil {
			return false, err
		}
		return image.Bounds().Dx() == image.Bounds().Dy(), nil
	}
	if isSquare, err := isSquarePhoto(fileInfos[0], ctx); err != nil {
		s.Error("Failed to parse captured photo: ", err)
	} else if !isSquare {
		s.Error("Captured photo is not square")
	}

	// Switch to portrait mode and take photo.
	if supported, err := app.PortraitModeSupported(ctx); err != nil {
		s.Error("Failed to determine whether portrait mode is supported: ", err)
	} else if supported {
		if err := app.SwitchMode(ctx, cca.Portrait); err != nil {
			s.Fatal("Failed to switch to portrait mode: ", err)
		}
		if _, err = app.TakeSinglePhoto(ctx, true); err != nil {
			s.Error("Failed to take portrait photo: ", err)
		}
	}


/*
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
	*/
}
