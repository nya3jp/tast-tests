// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cienet.com/cats/node"
	"cienet.com/cats/node/sdk"
	sdkUI "cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/ui/common"
	"chromiumos/tast/remote/bundles/mtbf/ui/svcutil"
	"chromiumos/tast/remote/cats"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF066GMediaFilesInFilesAPP,
		Desc:     "Android media files will be under Chrome Files App",
		Contacts: []string{"roger.cheng@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"cats.requestURL",
			"cats.nodeIP",
			"cats.nodePort",
			"cats.nodeGRPCPort",
		},
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.mtbf.ui.FilesAppService",
			"tast.mtbf.svcutil.CommService"},
	})
}

func MTBF066GMediaFilesInFilesAPP(ctx context.Context, s *testing.State) {

	// make sure "User has logged in"
	svcutil.Login(ctx, s)

	params, err := common.GetCatsRunParams(ctx, s)
	if err != nil {
		s.Fatal("CATS run parameter failed: ", err)
	}

	// New GRPC connection
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer cl.Close(ctx)

	// New GRPC Client
	cr := ui.NewFilesAppServiceClient(cl.Conn)

	params.CaseName = "MTBF066"
	params.TaskName = time.Now().Format("20060102150405000") + "_" + params.CaseName

	s.Logf("MTBF066 ARC Main with the task name [%s]", params.TaskName)
	caseResult, err := cats.DetachCaseRun(ctx, params, runMTBF066MainStage, nil)
	defer func() {
		newParams := params.DeepCopy()
		newParams.CaseName = "MTBF066_cleanup"
		newParams.TaskName = time.Now().Format("20060102150405000") + "_" + params.CaseName
		s.Logf("MTBF066 ARC Cleanup with the task name [%s]", newParams.TaskName)
		cats.DetachCaseRun(ctx, newParams, nil, runMTBF066CleanUpStage)
	}()

	s.Logf("CATS Node Case caseResult: %+v", caseResult)
	s.Logf("CATS Node Case error: %+v", err)

	if err != nil {
		s.Fatal("CATS case failed: ", err)
	}

	if caseResult == nil {
		// TODO mapping to mtbferrors error
		s.Fatal("CATS case result is nil")
	}

	result, ok := caseResult.(mtbf066MainStageResult)
	if !ok {
		s.Fatalf("Not mtbf066MainStageResult: [%+v]", caseResult)
	}

	// result := &catsMTBF066P1Result{Photo: "IMG_20200319_042351.jpg", Audio: "", Video: "IMG_20200319_042351.jpg"}

	s.Logf("[MTBF066g] Files from CATS: %s", result)

	downloadFiles, err := cr.GetAllFiles(ctx, &ui.FoldersRequest{Folders: []string{"Downloads"}})
	if err != nil {
		s.Fatal(mtbferrors.NewGRPCErr(err))
	}
	s.Logf("Downloads files: %d, %#v", len(downloadFiles.GetFiles()), downloadFiles.GetFiles())

	if !contains(downloadFiles.GetFiles(), result.Photo) {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, result.Photo))
	}
	if !contains(downloadFiles.GetFiles(), result.Video) {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, result.Video))
	}

	audioFiles, err := cr.GetAllFiles(ctx, &ui.FoldersRequest{Folders: []string{"Audio"}})
	if err != nil {
		s.Fatal(mtbferrors.NewGRPCErr(err))
	}
	s.Logf("Audio files: %#v", audioFiles.GetFiles())

	if !contains(audioFiles.GetFiles(), result.Audio) {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, result.Audio))
	}

}

func contains(a []string, x string) bool {
	if len(x) == 0 {
		return false
	}
	for _, n := range a {
		if strings.Contains(n, x) {
			return true
		}
	}
	return false
}

type mtbf066MainStageResult struct {
	Photo string `json:"photo"`
	Video string `json:"video"`
	Audio string `json:"audio"`
}

var (
	mutDevice              = "${deviceId0}"
	gcaAct                 = "com.google.android.apps.chromeos.camera.legacy.app.activity.main.CameraActivity"
	gcaPkg                 = "com.google.android.GoogleCameraArc"
	closeAppSelector       = "text=Close app"
	dismissSelector        = "text=DISMISS"
	okSelector             = "text=OK"
	noSelector             = "text=No"
	yesSelector            = "text=Yes"
	delSelector            = "text=Delete"
	gcaSelector            = "packagename=com.google.android.GoogleCameraArc"
	notFailOnNotMatch      = false
	gcaShutterSelector     = "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Shutter"
	gcaPhotoSwitchSelector = "ID=com.google.android.GoogleCameraArc:id/photo_switch_button"
	takeSnapshot           = true
	notTakeSnapshot        = false
	gcaThumbnailSelector   = "ID=com.google.android.GoogleCameraArc:id/thumbnail_button"
	gcaDeleteSelector      = "ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete::desc=Delete"
	logLevel               = node.LogLevel(6)
	noPhotosSelector       = "text=You have no photos"
	// gcaDeleteSelector      = "ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete"
	gcaShutterSelectorNoDesc  = "ID=com.google.android.GoogleCameraArc:id/shutter_button"
	gcaDetailsSelector        = "desc=Details"
	gcaTitleSelector          = "ID=android:id/text1::textstartwith=Title:"
	gcaStartRecordingSelector = "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Start Recording"
	gcaVideoSwitchSelector    = "ID=com.google.android.GoogleCameraArc:id/video_switch_button"
	defaultMainAct            = ".MainActivity"
	barPkg                    = "com.media.bestrecorder.audiorecorder"
	barNoThanksSelector       = "text=No, thanks"
	permissionAllowSelector   = "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button"
	barVoiceRecorderSelector  = "text=Voice Recorder"
	barRecordStartSelector    = "ID=com.media.bestrecorder.audiorecorder:id/btn_record_start"
	barPauseStartSelector     = "ID=com.media.bestrecorder.audiorecorder:id/layout_pause_record"
	barEdtFileNameSelector    = "id=com.media.bestrecorder.audiorecorder:id/edt_file_name"
	barTabFileListSelector    = "ID=com.media.bestrecorder.audiorecorder:id/tab_file_list"
	barDelFileListSelector    = "ID=com.media.bestrecorder.audiorecorder:id/btn_delete_list"
	barFileListSelector       = "ID=com.media.bestrecorder.audiorecorder:id/listview_file"
)

func runMTBF066MainStage(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
	// # 1. Use an Android camera app to take photos and videos.
	// # 2. Use an Android recorder app to take a sound clip.
	// take_photo()
	imageFileName := takePhoto(ctx, client, mutDevice)

	// record_video()
	videoFileName := recordVideo(ctx, client, mutDevice)

	// record_a_sound()
	audioFileName := recordSound(ctx, client, mutDevice)

	return mtbf066MainStageResult{
		Photo: imageFileName,
		Video: videoFileName,
		Audio: audioFileName,
	}, nil
}

func runMTBF066CleanUpStage(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
	// # 1. Open app and start recording audio.
	// actions.start_main_activity(mut_device, '.MainActivity', 'com.media.bestrecorder.audiorecorder', sleep=2000)
	client.StartMainActivity(mutDevice, defaultMainAct, barPkg).Do(ctx, service.Sleep(time.Second*2))

	// # Delete create audio on device.
	// delete_created_audio_file(mut_device)
	deleteCreatedAudioFile(ctx, client, mutDevice)

	// # Back to main page of recorder app.
	// press_cancel_button(mut_device, 2)
	pressCancelButton(ctx, client, mutDevice, 2)

	// is_no_thank = actions.uia_obj_event_wait(wait_time=2000, event_type="appear", device_id=mut_device, selector='text=No, thanks')
	isNoThank, _ := client.UIAObjEventWait(mutDevice,
		barNoThanksSelector, 2000, sdkUI.ObjEventTypeAppear).Do(ctx)

	// if is_no_thank:
	if isNoThank {
		// actions.uia_click(mut_device, selector='text=No, thanks', snapshot=True)
		client.UIAClick(mutDevice).Selector(barNoThanksSelector).Snapshot(takeSnapshot).Do(ctx)
		// press_cancel_button(mut_device)
		pressCancelButton(ctx, client, mutDevice, 1)
	}
	// is_main = actions.uia_obj_event_wait(mut_device, 'text=Voice Recorder', 2000, 'appear')
	isMain, _ := client.UIAObjEventWait(mutDevice,
		barVoiceRecorderSelector, 2000, sdkUI.ObjEventTypeAppear).Do(ctx)

	// if is_main:
	if isMain {
		// press_cancel_button(mut_device)
		pressCancelButton(ctx, client, mutDevice, 1)
	}

	// # Exit recorder app.
	// actions.click(mut_device, node_prop={'text': 'Yes'}, snapshot=True)
	client.UIAClick(mutDevice).Selector(yesSelector).Snapshot(takeSnapshot).Do(ctx)

	// actions.click(mut_device, node_prop={'text': 'No, thanks'}, snapshot=True)
	client.UIAClick(mutDevice).Selector(barNoThanksSelector).Snapshot(takeSnapshot).Do(ctx)

	// # Enter to app
	// actions.start_main_activity(mut_device, act='com.google.android.apps.chromeos.camera.legacy.app.activity.main.CameraActivity', pkg='com.google.android.GoogleCameraArc', sleep=2000)
	client.StartMainActivity(mutDevice, gcaAct, gcaPkg).Do(ctx, service.Sleep(time.Second*2))

	// # Click to enter to gallery
	// actions.uia_click(
	// 	mut_device, selector='ID=com.google.android.GoogleCameraArc:id/thumbnail_button', snapshot=True)
	clickGCAThumbnail(ctx, client, mutDevice)

	// # Delete create frame and video. Exit camera app
	// delete_created_frame_or_video(device_id=mut_device, times=3)
	deleteCreatedFrameOrVideo(ctx, client, mutDevice, 3)

	pressCancelButton(ctx, client, mutDevice, 3)
	client.UIAClick(mutDevice).Selector(dismissSelector).Do(ctx, service.Suppress())
	client.UIAClick(mutDevice).Selector(closeAppSelector).Do(ctx, service.Suppress())

	return nil, nil
}

func takePhoto(ctx context.Context, client sdk.DelegateClient, deviceID string) string {
	// global image_file_name
	// # Enter to app
	// actions.start_main_activity(mut_device, act='com.google.android.apps.chromeos.camera.legacy.app.activity.main.CameraActivity', pkg='com.google.android.GoogleCameraArc', sleep=2000)
	client.StartMainActivity(mutDevice, gcaAct, gcaPkg).Do(ctx, service.Sleep(time.Second*2))

	// # If App crash, close the dialogue window
	// actions.uia_click(mut_device, 'text=Close app', suppress=True)
	client.UIAClick(deviceID).Selector(closeAppSelector).Do(ctx, service.Suppress())

	// # Click OK
	// actions.uia_click(mut_device, selector='text=OK', suppress=True)
	client.UIAClick(deviceID).Selector(okSelector).Do(ctx, service.Suppress())

	// # Verify whether enter to Camera app
	// is_enetr_app = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='packagename=com.google.android.GoogleCameraArc', fail_on_not_match=False, snapshot=True)
	isEnterApp, _ := client.UIAObjEventWait(deviceID,
		gcaSelector, 6000, sdkUI.ObjEventTypeAppear).FailOnNotMatch(
		notFailOnNotMatch).Snapshot(takeSnapshot).Do(ctx)

	// if not is_enetr_app:
	//     helpers.fail('Fail to enter "Camera" app.', usererrcode='7001')
	if !(isEnterApp) {
		client.Fail(ctx, "Fail to enter \"Camera\" app.", true, 7001, "")
	}

	// # Delete all formrt created images or videos
	// delete_all_images_or_videos(mut_device)
	deleteAllImagesOrVideos(ctx, client, deviceID)

	// # Verify whethr in taking photo page. If not, Switch to photo page.
	// is_photo = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Shutter')
	isPhoto, _ := client.UIAObjEventWait(deviceID,
		gcaShutterSelector, 6000, sdkUI.ObjEventTypeAppear).Do(ctx)

	// if not is_photo:
	//     actions.uia_click(mut_device, selector='ID=com.google.android.GoogleCameraArc:id/photo_switch_button', snapshot=True)
	if !(isPhoto) {
		client.UIAClick(deviceID).Selector(gcaPhotoSwitchSelector).Snapshot(takeSnapshot).Do(ctx)
	}
	// # Take photo
	// click_shutter_button(device_id=mut_device)
	clickShutterButton(ctx, client, deviceID)

	// # Open captured frame to check.
	// open_captured_frame_or_recorded_video(device_id=mut_device)
	openCapturedFrameOrRecordedVideo(ctx, client, deviceID)

	// # Click info button and get file name
	// image_file_name = click_info_and_get_file_name(mut_device)
	imageFileName := clickInfoAndGetFileName(ctx, client, deviceID)

	// # Exit to main page of Camera app
	// press_cancel_button(mut_device, 2)
	pressCancelButton(ctx, client, deviceID, 2)
	return imageFileName
}

func deleteAllImagesOrVideos(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// # Click to enter to gallery
	// actions.uia_click(
	// 	device_id, selector='ID=com.google.android.GoogleCameraArc:id/thumbnail_button', snapshot=True)
	clickGCAThumbnail(ctx, client, deviceID)

	// is_enter_to_gallery = actions.uia_obj_event_wait(device_id, 'ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete::desc=Delete', 2000, 'appear')
	isEnter2Gallery, _ := client.UIAObjEventWait(deviceID,
		gcaDeleteSelector, 2000, sdkUI.ObjEventTypeAppear).FailOnNotMatch(notFailOnNotMatch).Do(ctx)

	// if is_enter_to_gallery:
	if isEnter2Gallery {
		// while True:
		for {
			// has_no_file = actions.uia_obj_event_wait(device_id, 'text=You have no photos', 1000, 'appear')
			hasNoFile, _ := client.UIAObjEventWait(deviceID,
				noPhotosSelector, 1000, sdkUI.ObjEventTypeAppear).Do(ctx)

			// if has_no_file:
			if hasNoFile {
				// 	break
				break
				// else:
			} else {
				client.UIAClick(deviceID).Selector(gcaDeleteSelector).Do(ctx)
			}
		}
		// actions.press(device_id, 'CANCEL', 1)
		client.Press(deviceID, sdkUI.OprKeyEventCANCEL).Times(1).Do(ctx)
		// else:
	} else {
		// actions.comments('No any image or video in gallery.')
		client.Comments("No any image or video in gallery.").Do(ctx)
	}
}

func clickShutterButton(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// actions.uia_click(device_id, selector='ID=com.google.android.GoogleCameraArc:id/shutter_button')
	client.UIAClick(deviceID).Selector(gcaShutterSelectorNoDesc).Do(ctx)
}

func openCapturedFrameOrRecordedVideo(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// # Click to enter to gallery
	// actions.uia_click(
	// 	device_id, selector='ID=com.google.android.GoogleCameraArc:id/thumbnail_button', snapshot=True)
	clickGCAThumbnail(ctx, client, deviceID)

	// is_enter_to_gallery = actions.uia_obj_event_wait(device_id, 'ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete::desc=Delete', 2000, 'appear')
	isEnter2Gallery, _ := client.UIAObjEventWait(deviceID,
		gcaDeleteSelector, 2000, sdkUI.ObjEventTypeAppear).FailOnNotMatch(notFailOnNotMatch).Do(ctx)

	// if not is_enter_to_gallery:
	if !isEnter2Gallery {
		// helpers.fail("Fail to enter gallery. Maybe app crashed or can't find any captured frame or recorded video.", usererrcode='7019')
		client.Fail(ctx,
			"Fail to enter gallery. Maybe app crashed or can't find any captured frame or recorded video.",
			true, 7019, "")
	}
}

func clickInfoAndGetFileName(ctx context.Context, client sdk.DelegateClient, deviceID string) string {
	// # Click info button.
	// actions.uia_click(device_id, selector='desc=Details')
	client.UIAClick(deviceID).Selector(gcaDetailsSelector).Do(ctx)

	// actions.get_widget_text(device_id, selector='ID=android:id/text1::textstartwith=Title:', variable_name='file_name')
	rsp, _ := client.GetWidgetText2(deviceID, gcaTitleSelector).Do(ctx)

	// file_name = helpers.get_var('file_name')[7:]
	// if file_name:
	// 	if file_name.startswith('IMG'):
	// 		file_name = file_name + '.jpg'
	// 	return file_name
	if len(rsp) > 0 {
		imageFile := (rsp)[7:]
		if strings.HasPrefix(imageFile, "IMG_") {
			imageFile = fmt.Sprintf("%s.jpg", imageFile)
		}
		client.Comments(fmt.Sprintf("Got an file: %s", imageFile)).Do(ctx)
		return imageFile
	}
	// else:
	// 	helpers.fail("Can't get the image/video file name taken in Camera app.", usererrcode='7019')
	client.Fail(ctx, "Can't get the image/video file name taken in Camera app.", true, 7019, "")

	return ""
}

func pressCancelButton(ctx context.Context, client sdk.DelegateClient, deviceID string, times int32) error {
	dut := utils.NewDevice(client, deviceID)
	return dut.PressCancelButton(ctx, int(times))
}

func recordVideo(ctx context.Context, client sdk.DelegateClient, deviceID string) string {
	// global video_file_name
	// is_video = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Start Recording')
	isVideo, _ := client.UIAObjEventWait(deviceID,
		gcaStartRecordingSelector, 6000, sdkUI.ObjEventTypeAppear).Do(ctx)

	if !isVideo {
		// actions.uia_click(mut_device, selector='ID=com.google.android.GoogleCameraArc:id/video_switch_button', snapshot=True)
		client.UIAClick(deviceID).Selector(gcaVideoSwitchSelector).Snapshot(takeSnapshot).Do(ctx, service.Sleep(time.Second*2))
	}
	// click_shutter_button(device_id=mut_device)
	clickShutterButton(ctx, client, deviceID)

	// actions.delay(5000)
	client.Delay(5000).Do(ctx)

	// click_shutter_button(device_id=mut_device)
	clickShutterButton(ctx, client, deviceID)

	// # Verify whether the video recording is stopped
	// is_stop = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Start Recording', snapshot=True)
	isStop, _ := client.UIAObjEventWait(deviceID,
		gcaStartRecordingSelector, 6000, sdkUI.ObjEventTypeAppear).Snapshot(takeSnapshot).Do(ctx)

	// if not is_stop:
	if !isStop {
		// click_shutter_button(device_id=mut_device)
		clickShutterButton(ctx, client, deviceID)
	}

	// # Open recorded video to check.
	// open_captured_frame_or_recorded_video(device_id=mut_device)
	openCapturedFrameOrRecordedVideo(ctx, client, mutDevice)

	// # Click info button and get file name
	// video_file_name = click_info_and_get_file_name(mut_device)
	videoFileName := clickInfoAndGetFileName(ctx, client, mutDevice)

	// # Exit to main page of Camera app
	// press_cancel_button(mut_device, 2)
	pressCancelButton(ctx, client, deviceID, 2)

	return videoFileName
}

func recordSound(ctx context.Context, client sdk.DelegateClient, deviceID string) string {
	// global audio_file_name
	// # 1. Open app and start recording audio.
	// actions.start_main_activity(mut_device, '.MainActivity', 'com.media.bestrecorder.audiorecorder', sleep=2000)
	client.StartMainActivity(deviceID, defaultMainAct, barPkg).Do(ctx, service.Sleep(time.Second*2))

	// actions.uia_click(mut_device, 'text=Close app', suppress=True)
	client.UIAClick(deviceID).Selector(closeAppSelector).Do(ctx, service.Suppress())

	// # Init Voice Recorder app. If button appears, click first.
	// is_no_thank = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='text=No, thanks')
	isNoThank, _ := client.UIAObjEventWait(deviceID,
		barNoThanksSelector, 6000, sdkUI.ObjEventTypeAppear).Snapshot(takeSnapshot).Do(ctx)

	// if is_no_thank:
	if !isNoThank {
		// press_cancel_button(mut_device)
		pressCancelButton(ctx, client, deviceID, 1)
	}

	// actions.click(mut_device, node_prop={'text': 'No'}, snapshot=True, suppress=True)
	client.UIAClick(deviceID).Selector(noSelector).Snapshot(takeSnapshot).Do(ctx, service.Suppress())

	// # Allow the permission
	// allow_permission(device_id=mut_device, selector='text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button')
	allowPermission(ctx, client, deviceID, permissionAllowSelector)

	// # Verify whether enter to correct app.
	// is_enetr_app = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='text=Voice Recorder', fail_on_not_match=False, snapshot=True)
	isEnterApp, _ := client.UIAObjEventWait(deviceID,
		barVoiceRecorderSelector, 6000, sdkUI.ObjEventTypeAppear,
	).FailOnNotMatch(notFailOnNotMatch).Snapshot(takeSnapshot).Do(ctx)

	// if not is_enetr_app:
	// 	helpers.fail('Fail to enter "Voice Recorder" app.', usererrcode='7001')
	if !isEnterApp {
		client.Fail(ctx, "Fail to enter \"Voice Recorder\" app.", true, 7001, "")
	}
	// # Start record audio
	// actions.uia_click(
	// 	mut_device, selector='ID=com.media.bestrecorder.audiorecorder:id/btn_record_start', snapshot=True, sleep=25000)
	client.UIAClick(deviceID).Selector(barRecordStartSelector).Snapshot(takeSnapshot).Do(
		ctx, service.Sleep(time.Second*25))

	// is_start = actions.uia_obj_event_wait(wait_time=6000, event_type="appear", device_id=mut_device, selector='ID=com.media.bestrecorder.audiorecorder:id/layout_pause_record')
	isStarted, _ := client.UIAObjEventWait(deviceID,
		barPauseStartSelector, 6000, sdkUI.ObjEventTypeAppear).Do(ctx)

	// if not is_start:
	if !isStarted {
		// helpers.fail('Fail to start record.', usererrcode='7018')
		client.Fail(ctx, "Fail to start record.", true, 7018, "")
	}
	// # Stop record
	// actions.uia_click(
	// 	mut_device, selector='ID=com.media.bestrecorder.audiorecorder:id/btn_record_start', snapshot=True)
	client.UIAClick(deviceID).Selector(barRecordStartSelector).Snapshot(takeSnapshot).Do(ctx)

	// # Get Audio file name.
	// audio_file_name = get_audio_file_name(device_id=mut_device)
	audioFileName := getAudioFileName(ctx, client, deviceID)

	// actions.uia_click(mut_device, selector='text=OK', snapshot=True)
	client.UIAClick(deviceID).Selector(okSelector).Do(ctx, service.Suppress())
	return audioFileName
}

func allowPermission(ctx context.Context, client sdk.DelegateClient, deviceID string, selector string) {
	// is_not_init = actions.uia_obj_event_wait(device_id=device_id, selector=selector, event_type='appear', wait_time=2000)
	isNotInit, _ := client.UIAObjEventWait(deviceID,
		selector, 2000, sdkUI.ObjEventTypeAppear).Do(ctx)

	// if is_not_init:
	if isNotInit {
		// for _ in range(5):
		for i := 0; i < 5; i++ {
			// # If target selector appears, click. And continue to check whether appears. If appears, click. Else, break.
			// actions.uia_click(device_id, selector=selector)
			client.UIAClick(deviceID).Selector(selector).Do(ctx)

			// is_not_init = actions.uia_obj_event_wait(device_id=device_id, selector=selector, event_type='appear', wait_time=3000)
			isNotInit, _ := client.UIAObjEventWait(deviceID,
				selector, 3000, sdkUI.ObjEventTypeAppear).Do(ctx)

			// if not is_not_init:
			if !isNotInit {
				// break
				break
			}
		}
	}
}

func getAudioFileName(ctx context.Context, client sdk.DelegateClient, deviceID string) string {
	// # Get the audio file name
	// actions.get_widget_text(device_id, selector='id=com.media.bestrecorder.audiorecorder:id/edt_file_name', variable_name='get_file_name')
	// get_file_name = helpers.get_var('get_file_name')
	rsp, _ := client.GetWidgetText2(deviceID, barEdtFileNameSelector).Do(ctx)

	// if get_file_name:
	// 	return get_file_name
	var audioFileName string
	if len(rsp) > 0 {
		audioFileName = rsp
		client.Comments(fmt.Sprintf("Got an file: %s", audioFileName)).Do(ctx)
	} else {
		// else:
		// helpers.fail("Can't get the recorded audio file name in Voice Recorder app.", usererrcode='7018')
		client.Fail(ctx, "Can't get the recorded audio file name in Voice Recorder app.",
			true, 7018, "")
	}
	return audioFileName
}

func deleteCreatedFrameOrVideo(ctx context.Context, client sdk.DelegateClient, deviceID string, times int) {
	// for _ in range(times):
	for i := 0; i < times; i++ {
		// 	actions.click(device_id, node_prop={
		// 					'resource-id': 'com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete'})
		client.UIAClick(deviceID).Selector(gcaDeleteSelector).Do(ctx)

		// actions.press(device_id, 'CANCEL', 1)
		client.Press(deviceID, sdkUI.OprKeyEventCANCEL).Times(1).Do(ctx)
	}
}

func deleteCreatedAudioFile(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// # Click to review all audio list
	// actions.uia_click(
	// 	device_id, selector='ID=com.media.bestrecorder.audiorecorder:id/tab_file_list', snapshot=True)
	client.UIAClick(deviceID).Selector(barTabFileListSelector).Snapshot(takeSnapshot).Do(ctx)

	// # Click delete button
	// is_delete = actions.uia_obj_event_wait(device_id, 'ID=com.media.bestrecorder.audiorecorder:id/btn_delete_list', 3000, 'appear')
	isDelete, _ := client.UIAObjEventWait(deviceID,
		barDelFileListSelector, 3000, sdkUI.ObjEventTypeAppear).Do(ctx)

	// if not is_delete:
	if !isDelete {
		// press_cancel_button(device_id)
		pressCancelButton(ctx, client, mutDevice, 1)
	}

	// actions.click(device_id, node_prop={
	// 				'resource-id': 'com.media.bestrecorder.audiorecorder:id/btn_delete_list'})
	client.UIAClick(deviceID).Selector(barDelFileListSelector).Do(ctx)

	// # Select created audio file
	// for i in range(3):
	for i := 0; i < 3; i++ {
		// 	actions.uia_click_list_item(device_id, target_selector='class=android.widget.FrameLayout::index=' + str(i), scroll_selector='ID=com.media.bestrecorder.audiorecorder:id/listview_file', snapshot=False)
		client.UIAClickListItem(deviceID,
			fmt.Sprintf("class=android.widget.FrameLayout::index=%d", i)).ScrollSelector(
			barFileListSelector).Snapshot(takeSnapshot).Do(ctx)
	}
	// # Delete the audio file
	// actions.click(device_id, node_prop={'text': 'Delete'})
	client.UIAClick(deviceID).Selector(delSelector).Do(ctx)

	// actions.click(device_id, node_prop={'text': 'OK'})
	client.UIAClick(deviceID).Selector(okSelector).Do(ctx)
}

func clickGCAThumbnail(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// // Maybe node detach mode is faster, so sleep 2s
	// client.Delay(ctx, &basic.DelayReq{
	// 	Millitime: node.Int64(3000),
	// })
	// actions.uia_click(
	// 	device_id, selector='ID=com.google.android.GoogleCameraArc:id/thumbnail_button', snapshot=True)
	client.UIAClick(deviceID).Selector(gcaThumbnailSelector).Snapshot(takeSnapshot).Do(ctx)
}
