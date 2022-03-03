// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// "Precondition:
// Login to ChromeOS device
// USB camera

// Procedure:
// 1. Launch ChromeOS camera app
//        -- Verify default camera is the front camera when the device have multiple cameras

// 2. Plug the external Camera and select the plugged external camera using the camera switch button.
//        -- Verify camera app preview should switch to USB camera video feed.

// 3. Take a photo and capture video with an external camera.
//        -- Verify the photo and video taken with external camera are clear and looks good

// 4.  Unplug the external camera.
//     -- Verify camera app should automatically switch to the default front camera.
//     --  Playback the video and verify it plays as expected"

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Webcams1ExternalCam,
		Desc:         "CCA app with external camera",
		Contacts:     []string{"allion-sw@allion.com"},
		Timeout:      4 * time.Minute,
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
	})
}

func Webcams1ExternalCam(ctx context.Context, s *testing.State) {
	// "Precondition:
	// Login to ChromeOS device
	// USB camera
	s.FixtValue().(cca.FixtureData).SetDebugParams(cca.DebugParams{SaveCameraFolderWhenFail: true})
	cr := s.FixtValue().(cca.FixtureData).Chrome
	app := s.FixtValue().(cca.FixtureData).App()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Procedure:
	// 1. Launch ChromeOS camera app
	// 	   -- Verify default camera is the front camera when the device have multiple cameras
	if err := webcams1ExternalCamStep1(ctx, s, app); err != nil {
		s.Fatal("Failed to execute step1: ", err)
	}

	// refer to -> cca_ui_multi_camera.go
	// 2. Plug the external Camera and select the plugged external camera using the camera switch button.
	// 	   -- Verify camera app preview should switch to USB camera video feed.
	if err := webcams1ExternalCamStep2(ctx, s, app); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// refer to => cca_ui_record_video.go & cca_ui_take_picture.go
	// 3. Take a photo and capture video with an external camera.
	// 	   -- Verify the photo and video taken with external camera are clear and looks good
	if err := webcams1ExternalCamStep3(ctx, s, cr, tconn, app); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// 4.  Unplug the external camera.
	// 	-- Verify camera app should automatically switch to the default front camera.
	// 	--  Playback the video and verify it plays as expected"
	if err := webcams1ExternalCamStep4(ctx, s, app); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}
}

func webcams1ExternalCamStep1(ctx context.Context, s *testing.State, app *cca.App) error {

	// when the device have multiple cameras
	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		s.Fatal("Can't get number of cameras: ", err)
	}
	s.Logf("Num of camera is %d", numCameras)

	if numCameras <= 1 {
		return errors.Errorf("Chromebook shall have multiple cameras, at least 2 , got %d", numCameras)
	}

	// Verify default camera is the front camera
	facing, err := app.GetFacing(ctx)
	if err != nil {
		return errors.Wrap(err, "get facing failed")
	}

	if facing != cca.FacingFront {
		return errors.Errorf("failed to verify default camera is the front camera, want %s, got %s", cca.FacingFront, facing)
	}

	return nil
}

// webcams1ExternalCamStep2 refer to -> cca_ui_multi_camera.go
func webcams1ExternalCamStep2(ctx context.Context, s *testing.State, app *cca.App) error {

	//plug in external camera
	// if err := utils.DoSwitchFixture(ctx, s, utils.CameraType, utils.CameraIndex, utils.ActionPlugin, false); err != nil {
	// 	return err
	// }

	// select external camrea
	if err := app.SwitchCamera(ctx); err != nil {
		s.Fatal("Switch camera failed: ", err)
	}

	if err := app.CheckFacing(ctx, cca.FacingExternal); err != nil {
		return err
	}

	// -- Verify camera app preview should switch to USB camera video feed.
	// just check camera video or photo in step 3

	return nil
}

// webcams1ExternalCamStep3 refer to => cca_ui_record_video.go & cca_ui_take_picture.go
func webcams1ExternalCamStep3(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, app *cca.App) error {

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Verify the photo and video taken with external camera are clear and looks good
	// play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	// send "f" to enter youtube full screen
	if err := kb.Accel(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to let youtube into full screen")
	}

	if err := checkPhotoLooksGood(ctx, s, app); err != nil {
		return err
	}

	if err := checkVideoLooksGood(ctx, s, cr, tconn, app); err != nil {
		return err
	}

	// send "esc" to chromebook exit youtube full screen
	if err := kb.Accel(ctx, "esc"); err != nil {
		return errors.Wrap(err, "failed to let youtube exit full screen")
	}

	// get youtube window
	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	}

	// close youtube window
	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close youtube")
	}

	return nil
}

func webcams1ExternalCamStep4(ctx context.Context, s *testing.State, app *cca.App) error {

	// 4.  Unplug the external camera.
	// if err := utils.DoSwitchFixture(ctx, s, utils.CameraType, utils.CameraIndex, utils.ActionUnplug, false); err != nil {
	// 	return err
	// }

	// 	-- Verify camera app should automatically switch to the default front camera.
	if err := app.CheckFacing(ctx, cca.FacingFront); err != nil {
		return err
	}

	return nil

}

func checkPhotoLooksGood(ctx context.Context, s *testing.State, app *cca.App) error {

	// take a photo
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		s.Error("Failed to switch to photo mode: ", err)
	}

	info, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to take photo")
	}

	photoPath, err := app.FilePathInSavedDir(ctx, info[0].Name())
	if err != nil {
		return errors.Wrap(err, "failed to get file path")
	}

	if err := utils.CopyFileToServer(ctx, s, photoPath); err != nil {
		return err
	}

	s.Logf("Photo path is %s", photoPath)

	return nil
}

func checkVideoLooksGood(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, app *cca.App) error {

	// switch video mode
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Error("Failed to switch to photo mode: ", err)
	}

	file, err := app.RecordVideo(ctx, cca.TimerOn, time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to record video")
	}

	videoPath, err := app.FilePathInSavedDir(ctx, file.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get video path")
	}

	// 	--  Playback the video and verify it plays as expected"
	if err := cca.CheckVideoProfile(videoPath, cca.ProfileH264High); err != nil {
		return errors.Wrap(err, "failed to check video profile ")
	}

	if err := utils.CopyFileToServer(ctx, s, videoPath); err != nil {
		return err
	}

	return nil
}
