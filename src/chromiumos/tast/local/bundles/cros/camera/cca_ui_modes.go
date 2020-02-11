// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"image/jpeg"
	"os"
	"path/filepath"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIModes,
		Desc:         "Opens CCA and verifies the use cases of mode selector and portrait, square modes",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js", "human_face.y4m"},
	})
}

func CCAUIModes(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs(
		"--use-fake-ui-for-media-stream",
		"--use-fake-device-for-media-stream",
		"--use-file-for-fake-video-capture="+s.DataPath("human_face.y4m")))
	if err != nil {
		s.Fatal("Failed to open chrome: ", err)
	}
	defer cr.Close(ctx)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir())
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	// Switch to square mode and take photo.
	if err := app.SwitchMode(ctx, cca.Square); err != nil {
		s.Fatal("Failed to switch to square mode: ", err)
	}
	fileInfos, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		s.Error("Failed to take square photo: ", err)
	}

	isSquarePhoto := func(info os.FileInfo, ctx context.Context, app *cca.App) (bool, error) {
		path, err := cca.GetSavedDir(ctx, cr)
		if err != nil {
			return false, err
		}
		path = filepath.Join(path, info.Name())
		file, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer file.Close()
		image, err := jpeg.Decode(file)
		if err != nil {
			return false, err
		}
		return image.Bounds().Dx() == image.Bounds().Dy(), nil
	}
	if isSquare, err := isSquarePhoto(fileInfos[0], ctx, app); err != nil {
		s.Error("Failed to parse captured photo: ", err)
	} else if !isSquare {
		s.Error("Captured photo is not square")
	}

	// Switch to portrait mode and take photo.
	// TODO(shik): Move portrait mode testing to isolated test so that it only
	// runs on devices with portrait mode support. crbug.com/988732
	if supported, err := app.PortraitModeSupported(ctx); err != nil {
		s.Error("Failed to determine whether portrait mode is supported: ", err)
	} else if supported {
		if err := app.SwitchMode(ctx, cca.Portrait); err != nil {
			s.Fatal("Failed to switch to portrait mode: ", err)
		}
		if _, err = app.TakeSinglePhoto(ctx, cca.TimerOff); err != nil {
			s.Error("Failed to take portrait photo: ", err)
		}
	}
}
