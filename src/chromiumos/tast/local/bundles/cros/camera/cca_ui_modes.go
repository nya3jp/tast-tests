// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"image/jpeg"
	"os"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIModes,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Opens CCA and verifies the use cases of mode selector and portrait, square modes",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func CCAUIModes(ctx context.Context, s *testing.State) {
	app := s.FixtValue().(cca.FixtureData).App()

	exist, err := app.Exist(ctx, cca.SquareModeButton)
	if err != nil {
		s.Fatal("Failed to check existence of square mode button: ", err)
	}
	if !exist {
		// Acceptable since there is no square mode in the new UI design.
		return
	}

	// TODO(b/215484798): Remove the old logic and implement the aspect ratio
	// settings check in CCAUIResolutions or CCAUISettings.

	// Switch to square mode and take photo.
	if err := app.SwitchMode(ctx, cca.Square); err != nil {
		s.Fatal("Failed to switch to square mode: ", err)
	}
	fileInfos, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		s.Error("Failed to take square photo: ", err)
	}

	isSquarePhoto := func(info os.FileInfo, ctx context.Context, app *cca.App) (bool, error) {
		path, err := app.FilePathInSavedDir(ctx, info.Name())
		if err != nil {
			return false, err
		}
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
}
