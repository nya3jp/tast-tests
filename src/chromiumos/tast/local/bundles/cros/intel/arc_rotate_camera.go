// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCRotateCamera,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "ARC++ user-facing Camera record/Camera capture the video while rotating the DUT",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "ccaTestBridgeReadyWithArc",
		Timeout:      arc.BootTimeout + 2*time.Minute,
		Params: []testing.Param{{
			Name: "photo",
			Val:  cca.Photo,
		}, {
			Name: "video",
			Val:  cca.Video,
		}},
	})
}

func ARCRotateCamera(ctx context.Context, s *testing.State) {
	startApp := s.FixtValue().(cca.FixtureData).StartApp
	stopApp := s.FixtValue().(cca.FixtureData).StopApp
	cr := s.FixtValue().(cca.FixtureData).Chrome
	mode := s.Param().(cca.Mode)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	screen, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get internal display info: ", err)
	}
	// Ensure returning back to the normal rotation at the end.
	defer display.SetDisplayRotationSync(cleanupCtx, tconn, screen.ID, display.Rotate0)

	app, err := startApp(ctx)
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}

	defer func(cleanupCtx context.Context) {
		if err := stopApp(cleanupCtx, s.HasError()); err != nil {
			s.Fatal("Failed to close CCA: ", err)
		}
	}(cleanupCtx)

	if mode == cca.Video {
		if err := rotateAndRecordVideo(ctx, tconn, screen, app); err != nil {
			s.Fatal("Failed to rotate device's display and record video: ", err)
		}
	} else if mode == cca.Photo {
		if err := rotateAndTakePhoto(ctx, tconn, screen, app); err != nil {
			s.Fatal("Failed to rotate device's display and take photo: ", err)
		}
	}

}

// rotateAndTakePhoto rotates the camera and take pictures.
func rotateAndTakePhoto(ctx context.Context, tconn *chrome.TestConn, info *display.Info, app *cca.App) error {
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		return errors.Wrap(err, "failed to switch to photo mode")
	}
	for _, rotation := range []display.RotationAngle{
		display.Rotate90,
		display.Rotate180,
		display.Rotate270,
		display.Rotate0,
	} {
		testing.ContextLog(ctx, "Rotating to: ", rotation)
		if err := display.SetDisplayRotationSync(ctx, tconn, info.ID, rotation); err != nil {
			return errors.Wrapf(err, "failed rotating display to %v", rotation)
		}
		fileInfos, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
		if err != nil {
			return errors.Wrap(err, "failed to take photo")
		}
		photoPath, err := app.FilePathInSavedDir(ctx, fileInfos[0].Name())
		if err != nil {
			return errors.Wrap(err, "failed to get captured photo path")
		}
		if photoPath == "" {
			return errors.New("failed captured photo path is empty")
		}
	}
	return nil
}

// rotateAndRecordVideo rotates the camera and records video.
func rotateAndRecordVideo(ctx context.Context, tconn *chrome.TestConn, info *display.Info, app *cca.App) error {
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		return errors.Wrap(err, "failed to switch to video mode")
	}
	start, err := app.StartRecording(ctx, cca.TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to start recording")
	}
	for _, rotation := range []display.RotationAngle{
		display.Rotate90,
		display.Rotate180,
		display.Rotate270,
		display.Rotate0,
	} {
		testing.ContextLog(ctx, "Rotating to: ", rotation)
		if err := display.SetDisplayRotationSync(ctx, tconn, info.ID, rotation); err != nil {
			return errors.Wrapf(err, "failed rotating display to %v", rotation)
		}
	}
	fileInfo, _, err := app.StopRecording(ctx, cca.TimerOff, start)
	if err != nil {
		return errors.Wrap(err, "failed to stop recording")
	}
	videoPath, err := app.FilePathInSavedDir(ctx, fileInfo.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get captured video path")
	}
	if videoPath == "" {
		return errors.New("failed captured video path is empty")
	}
	return nil
}
