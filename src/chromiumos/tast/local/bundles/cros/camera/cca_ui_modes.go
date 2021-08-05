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
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIModes,
		Desc:         "Opens CCA and verifies the use cases of mode selector and portrait, square modes",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:camera-libcamera"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIModes(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tb, err := testutil.NewTestBridge(ctx, cr, testutil.UseRealCamera)
	if err != nil {
		s.Fatal("Failed to construct test bridge: ", err)
	}
	defer tb.TearDown(ctx)

	if err := cca.ClearSavedDir(ctx, cr); err != nil {
		s.Fatal("Failed to clear saved directory: ", err)
	}

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")}, s.OutDir(), tb)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer func(ctx context.Context) {
		if err := app.Close(ctx); err != nil {
			s.Error("Failed to close app: ", err)
		}
	}(ctx)

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

	// For portrait mode, currently we only check for the existence of the
	// reference photo but don't check for the processed output since we cannot
	// guarantee there is a human face in the frame content.
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
