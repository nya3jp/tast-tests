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

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

// camera
const (
	cameraType  = "TYPEA_Switch"
	cameraIndex = "ID1"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Webcam1ExternalCam,
		Desc:         "CCA app with external camera",
		Contacts:     []string{"allion-sw@allion.com"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		Fixture:      "ccaLaunched",
		VarDeps:      []string{"FixtureWebUrl"},
	})
}

func Webcam1ExternalCam(ctx context.Context, s *testing.State) {
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
	if err := webcam1ExternalCamStep1(ctx, s, app); err != nil {
		s.Fatal("Failed to execute step1: ", err)
	}

	// refer to -> cca_ui_multi_camera.go
	// 2. Plug the external Camera and select the plugged external camera using the camera switch button.
	// 	   -- Verify camera app preview should switch to USB camera video feed.
	if err := webcam1ExternalCamStep2(ctx, s, app); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// refer to => cca_ui_record_video.go & cca_ui_take_picture.go
	// 3. Take a photo and capture video with an external camera.
	// 	   -- Verify the photo and video taken with external camera are clear and looks good
	if err := webcam1ExternalCamStep3(ctx, s, cr, tconn, app); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// 4.  Unplug the external camera.
	// 	-- Verify camera app should automatically switch to the default front camera.
	// 	--  Playback the video and verify it plays as expected"
	if err := webcam1ExternalCamStep4(ctx, s, app); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}
}

func webcam1ExternalCamStep1(ctx context.Context, s *testing.State, app *cca.App) error {

	s.Log("Step 1 - Launch ChromeOS camera app")

	s.Log("Verify default camera is the front camera when the device have multiple cameras")

	// when the device have multiple cameras
	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get number of cameras")
	}

	if numCameras <= 1 {
		return errors.Errorf("Chromebook shall have multiple cameras, at least 2 , got %d", numCameras)
	}

	// Verify default camera is the front camera
	if err := app.CheckFacing(ctx, cca.FacingFront); err != nil {
		return err
	}

	return nil
}

// webcam1ExternalCamStep2 refer to -> cca_ui_multi_camera.go
func webcam1ExternalCamStep2(ctx context.Context, s *testing.State, app *cca.App) error {

	s.Log("Step 2 - Plug the external Camera and select the plugged external camera using the camera switch button")

	// plug in external camera
	if err := utils.ControlFixture(ctx, s, cameraType, cameraIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in external camera")
	}

	// select external camrea
	if err := app.SwitchCamera(ctx); err != nil {
		return errors.Wrap(err, "failed to switch camera")
	}

	// check
	if err := app.CheckFacing(ctx, cca.FacingExternal); err != nil {
		return err
	}

	// -- Verify camera app preview should switch to USB camera video feed.
	// just check camera video or photo in step 3

	return nil
}

// webcam1ExternalCamStep3 refer to => cca_ui_record_video.go & cca_ui_take_picture.go
func webcam1ExternalCamStep3(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, app *cca.App) error {

	s.Log("Step 3 - Verify the photo and video taken with external camera are clear and looks good")

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
	// open chrome to url
	conn, err := cr.NewConn(ctx, utils.YouTubeURL, browser.WithNewWindow())
	if err != nil {
		return errors.Wrap(err, "could not get youTube request")
	}

	// close it when finished
	defer conn.Close()

	// check window info is correct
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsActive && strings.Contains(w.Title, utils.VideoTitle) && w.IsVisible == true
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "app window not focused after clicking shelf icon")
	}

	// send "f" to enter youtube full screen
	if err := kb.Accel(ctx, "f"); err != nil {
		return errors.Wrap(err, "failed to let youtube into full screen")
	}

	if err := checkPhotoLooksGood(ctx, s, app); err != nil {
		return errors.Wrap(err, "failed to check photo looks good")
	}

	if err := checkVideoLooksGood(ctx, s, cr, tconn, app); err != nil {
		return errors.Wrap(err, "failed to check video looks good")
	}

	// send "esc" to chromebook exit youtube full screen
	if err := kb.Accel(ctx, "esc"); err != nil {
		return errors.Wrap(err, "failed to let youtube exit full screen")
	}

	if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
		return errors.Wrap(err, "failed to close youtube")
	}

	return nil
}

func webcam1ExternalCamStep4(ctx context.Context, s *testing.State, app *cca.App) error {

	s.Log("Step 4 - Unplug the external camera")

	// 4.  Unplug the external camera.
	if err := utils.ControlFixture(ctx, s, cameraType, cameraIndex, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "failed to unplug the external camera")
	}

	s.Log("Verify camera app should automatically switch to the default front camera")

	// 	-- Verify camera app should automatically switch to the default front camera.
	if err := app.CheckFacing(ctx, cca.FacingFront); err != nil {
		return err
	}

	return nil

}

func checkPhotoLooksGood(ctx context.Context, s *testing.State, app *cca.App) error {

	// switch to photo mode
	if err := app.SwitchMode(ctx, cca.Photo); err != nil {
		s.Error("Failed to switch to photo mode: ", err)
	}

	// take a photo
	info, err := app.TakeSinglePhoto(ctx, cca.TimerOff)
	if err != nil {
		return errors.Wrap(err, "failed to take photo")
	}

	// get photo path
	photoPath, err := app.FilePathInSavedDir(ctx, info[0].Name())
	if err != nil {
		return errors.Wrap(err, "failed to get file path")
	}

	// upload file to wwcb server
	uploadPath, err := utils.UploadFile(ctx, photoPath)
	if err != nil {
		return errors.Wrap(err, "failed to upload file to wwcb server")
	}

	// compare pic with golden sample on wwcb server
	if err := compareWebcamPic(s, uploadPath); err != nil {
		return errors.Wrap(err, "failed to compare webcam pic")
	}

	return nil
}

func checkVideoLooksGood(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn, app *cca.App) error {

	// switch video mode
	if err := app.SwitchMode(ctx, cca.Video); err != nil {
		s.Error("Failed to switch to photo mode: ", err)
	}

	// recode video
	file, err := app.RecordVideo(ctx, cca.TimerOn, time.Minute)
	if err != nil {
		return errors.Wrap(err, "failed to record video")
	}

	// get video file path
	videoPath, err := app.FilePathInSavedDir(ctx, file.Name())
	if err != nil {
		return errors.Wrap(err, "failed to get video path")
	}

	// 	--  Playback the video and verify it plays as expected"
	if err := cca.CheckVideoProfile(videoPath, cca.ProfileH264High); err != nil {
		return errors.Wrap(err, "failed to check video profile ")
	}

	// upload file to wwcb server
	uploadPath, err := utils.UploadFile(ctx, videoPath)
	if err != nil {
		return errors.Wrap(err, "failed to upload file to wwcb server")
	}

	// compare video with golden sample on wwcb server
	if err := compareWebcamVideo(s, uploadPath); err != nil {
		return errors.Wrap(err, "failed to compare webcam video")
	}

	return nil
}

// compareWebcamPic compare webcam pic
func compareWebcamPic(s *testing.State, filepath string) error {

	WWCBServerURL, ok := s.Var("FixtureWebURL")
	if !ok {
		return errors.Errorf("Runtime variable %s is not provided", WWCBServerURL)
	}

	// construct URL
	URL := fmt.Sprintf("%s/api/compare_webcam_pic?filepath=%s",
		WWCBServerURL,
		filepath)

	s.Log("request:", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", URL)
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})

	if m["resultCode"] != "0" || m["resultTxt"] != "success" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}

// compareWebcamVideo compare webcam pic
func compareWebcamVideo(s *testing.State, filepath string) error {

	WWCBServerURL, ok := s.Var("FixtureWebURL")
	if !ok {
		return errors.Errorf("Runtime variable %s is not provided", WWCBServerURL)
	}

	// construct URL
	URL := fmt.Sprintf("%s/api/compare_webcam_video?filepath=%s",
		WWCBServerURL,
		filepath)

	s.Log("request: ", URL)

	// send request
	res, err := http.Get(URL)
	if err != nil {
		return errors.Wrapf(err, "failed to get response: %s", URL)
	}
	// dispose when finished
	defer res.Body.Close()

	// get response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read all response")
	}

	// parse response
	var data interface{} // TopTracks
	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "failed to parse data to json")
	}

	s.Log("response: ", data)

	// check response
	m := data.(map[string]interface{})

	if m["resultCode"] != "0" || m["resultTxt"] != "success" {
		return errors.New("failed to get correct response: ")
	}

	return nil
}
