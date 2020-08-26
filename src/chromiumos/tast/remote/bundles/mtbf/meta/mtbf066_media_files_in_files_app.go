// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF066MediaFilesInFilesAPP,
		Desc:     "Android media files will be under Chrome Files App",
		Contacts: []string{"roger.cheng@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"meta.requestURL",
		},
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_p"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
			"tast.mtbf.ui.FilesApp",
		},
	})
}

// MTBF066MediaFilesInFilesAPP Run MTBF066 case
func MTBF066MediaFilesInFilesAPP(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF066MediaFilesInFilesAPP",
		Description: "Android media files will be under Chrome Files App",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		type action struct {
			Desc string
			Fn   func(context.Context, sdk.DelegateClient) (string, error)
		}
		var (
			actions = []action{
				{Desc: "taking photo", Fn: takePhoto},
				{Desc: "recording video", Fn: recordVideo},
				{Desc: "recording audio", Fn: recordSound},
			}
			paths = make([]string, 0, len(actions))
		)

		// create media files by ARC++ apps
		for _, act := range actions {
			testing.ContextLog(ctx, "Start ", act.Desc)
			name, err := act.Fn(ctx, client)
			if err != nil {
				common.Fatal(ctx, s, err)
			}
			paths = append(paths, name)

			testing.ContextLog(ctx, "Got filename: ", name)
			testing.Sleep(ctx, time.Second)
		}
		paths[2] = "Recorders/" + paths[2] // records stored in directory "Recorders"

		// init gRPC client
		c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer c.Close(ctx)

		app := pb.NewFilesAppClient(c.Conn)
		defer app.Close(ctx, &empty.Empty{})

		// check if files are visible in FilesApp
		for _, path := range paths {
			if _, err = app.SelectInDownloads(ctx, &pb.FileRequest{Path: path}); err != nil {
				common.Fatal(ctx, s, err)
			}
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {

		testing.ContextLog(ctx, "Start case cleanup")
		runMTBF066CleanUpStage(ctx, client)

		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

var (
	mutDevice                 = "${deviceId0}"
	gcaAct                    = "com.google.android.apps.chromeos.camera.legacy.app.activity.main.CameraActivity"
	gcaPkg                    = "com.google.android.GoogleCameraArc"
	closeAppSelector          = "text=Close app"
	dismissSelector           = "text=DISMISS"
	okSelector                = "text=OK"
	noSelector                = "text=No"
	containsNoSelector        = "textcontains=No"
	yesSelector               = "text=Yes"
	delSelector               = "text=Delete"
	gcaSelector               = "packagename=com.google.android.GoogleCameraArc"
	notFailOnNotMatch         = false
	gcaShutterSelector        = "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Shutter"
	gcaPhotoSwitchSelector    = "ID=com.google.android.GoogleCameraArc:id/photo_switch_button"
	takeSnapshot              = true
	notTakeSnapshot           = false
	gcaThumbnailSelector      = "ID=com.google.android.GoogleCameraArc:id/thumbnail_button"
	gcaDeleteSelector         = "ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete::desc=Delete"
	logLevel                  = sdk.LogLevel(6)
	noPhotosSelector          = "text=You have no photos"
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

// runMTBF066CleanUpStage runs cleanup stage
func runMTBF066CleanUpStage(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
	// 1. Open app and start recording audio.
	client.StartMainActivity(mutDevice, defaultMainAct, barPkg).Do(ctx, service.Sleep(time.Second*2))

	// Delete create audio on device.
	deleteCreatedAudioFile(ctx, client, mutDevice)

	// Back to main page of recorder app.
	pressCancelButton(ctx, client, mutDevice, 2)

	isNoThank, _ := client.UIAObjEventWait(mutDevice,
		barNoThanksSelector, 2000, ui.ObjEventTypeAppear).Do(ctx)

	if isNoThank {
		client.UIAClick(mutDevice).Selector(barNoThanksSelector).Snapshot(takeSnapshot).Do(ctx)
		pressCancelButton(ctx, client, mutDevice, 1)
	}
	isMain, _ := client.UIAObjEventWait(mutDevice,
		barVoiceRecorderSelector, 2000, ui.ObjEventTypeAppear).Do(ctx)

	if isMain {
		pressCancelButton(ctx, client, mutDevice, 1)
	}

	// Exit recorder app.
	client.UIAClick(mutDevice).Selector(yesSelector).Snapshot(takeSnapshot).Do(ctx)

	client.UIAClick(mutDevice).Selector(barNoThanksSelector).Snapshot(takeSnapshot).Do(ctx)

	// Enter to app
	client.StartMainActivity(mutDevice, gcaAct, gcaPkg).Do(ctx, service.Sleep(time.Second*2))

	// Click to enter to gallery
	clickGCAThumbnail(ctx, client, mutDevice)

	// Delete create frame and video. Exit camera app
	deleteCreatedFrameOrVideo(ctx, client, mutDevice, 3)

	pressCancelButton(ctx, client, mutDevice, 3)
	client.UIAClick(mutDevice).Selector(dismissSelector).Do(ctx)
	client.UIAClick(mutDevice).Selector(closeAppSelector).Do(ctx)

	return nil, nil
}

func takePhoto(ctx context.Context, client sdk.DelegateClient) (string, error) {
	// global image_file_name
	client.StartMainActivity(mutDevice, gcaAct, gcaPkg).Do(ctx, service.Sleep(time.Second*2))

	// If App crash, close the dialogue window
	client.UIAClick(mutDevice).Selector(closeAppSelector).Do(ctx, service.Suppress())

	// Click OK
	client.UIAClick(mutDevice).Selector(okSelector).Do(ctx, service.Suppress())

	// Verify whether enter to Camera app
	isEnterApp, _ := client.UIAObjEventWait(mutDevice,
		gcaSelector, 6000, ui.ObjEventTypeAppear).FailOnNotMatch(
		notFailOnNotMatch).Snapshot(takeSnapshot).Do(ctx)

	if !(isEnterApp) {
		return "", mtbferrors.New(mtbferrors.EnterCameraApp, nil)
	}

	// Delete all formrt created images or videos
	deleteAllImagesOrVideos(ctx, client, mutDevice)

	// Verify whethr in taking photo page. If not, Switch to photo page.
	isPhoto, _ := client.UIAObjEventWait(mutDevice,
		gcaShutterSelector, 6000, ui.ObjEventTypeAppear).Do(ctx)

	if !(isPhoto) {
		client.UIAClick(mutDevice).Selector(gcaPhotoSwitchSelector).Snapshot(takeSnapshot).Do(ctx)
	}
	// Take photo
	clickShutterButton(ctx, client, mutDevice)

	// Open captured frame to check.
	openCapturedFrameOrRecordedVideo(ctx, client, mutDevice)

	// Click info button and get file name
	imageFileName, mtbferror := clickInfoAndGetFileName(ctx, client, mutDevice)
	if mtbferror != nil {
		return "", mtbferror
	}

	// Exit to main page of Camera app
	pressCancelButton(ctx, client, mutDevice, 2)
	return imageFileName, nil
}

func deleteAllImagesOrVideos(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// Click to enter to gallery
	clickGCAThumbnail(ctx, client, deviceID)

	isEnter2Gallery, _ := client.UIAObjEventWait(deviceID,
		gcaDeleteSelector, 2000, ui.ObjEventTypeAppear).FailOnNotMatch(notFailOnNotMatch).Do(ctx)

	if isEnter2Gallery {
		for {
			hasNoFile, _ := client.UIAObjEventWait(deviceID,
				noPhotosSelector, 1000, ui.ObjEventTypeAppear).Do(ctx)

			if hasNoFile {
				break
			} else {
				client.UIAClick(deviceID).Selector(gcaDeleteSelector).Do(ctx)
			}
		}
		client.Press(deviceID, ui.OprKeyEventCANCEL).Times(1).Do(ctx)
	} else {
		client.Comments("No any image or video in gallery.").Do(ctx)
	}
}

func clickShutterButton(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	client.UIAClick(deviceID).Selector(gcaShutterSelectorNoDesc).Do(ctx)
}

func openCapturedFrameOrRecordedVideo(ctx context.Context, client sdk.DelegateClient, deviceID string) error {
	// Click to enter to gallery
	clickGCAThumbnail(ctx, client, deviceID)

	isEnter2Gallery, _ := client.UIAObjEventWait(deviceID,
		gcaDeleteSelector, 2000, ui.ObjEventTypeAppear).FailOnNotMatch(notFailOnNotMatch).Do(ctx)

	if !isEnter2Gallery {
		return mtbferrors.New(mtbferrors.EnterGallery, nil)
	}
	return nil
}

func clickInfoAndGetFileName(ctx context.Context, client sdk.DelegateClient, deviceID string) (string, error) {
	// click info button.
	client.UIAClick(deviceID).Selector(gcaDetailsSelector).Do(ctx)

	name, _ := client.GetWidgetText2(deviceID, gcaTitleSelector).Do(ctx)
	if len(name) == 0 {
		return "", mtbferrors.New(mtbferrors.CanootGetImgOrVideoFileName, nil)
	}

	name = strings.TrimPrefix(name, "Title: ")
	if strings.HasPrefix(name, "IMG_") { // append ".jpg" if it's an image
		name = name + ".jpg"
	}
	client.Comments(fmt.Sprintf("Got an file: %s", name)).Do(ctx)
	return name, nil
}

func pressCancelButton(ctx context.Context, client sdk.DelegateClient, deviceID string, times int32) {
	client.Press(deviceID, ui.OprKeyEventCANCEL).Times(times).Do(ctx)
}

func recordVideo(ctx context.Context, client sdk.DelegateClient) (string, error) {
	isVideo, _ := client.UIAObjEventWait(mutDevice,
		gcaStartRecordingSelector, 6000, ui.ObjEventTypeAppear).Do(ctx)

	if !isVideo {
		client.UIAClick(mutDevice).Selector(gcaVideoSwitchSelector).Snapshot(takeSnapshot).Do(ctx, service.Sleep(time.Second*2))
	}
	clickShutterButton(ctx, client, mutDevice)

	client.Delay(5000).Do(ctx)

	clickShutterButton(ctx, client, mutDevice)

	isStop, _ := client.UIAObjEventWait(mutDevice,
		gcaStartRecordingSelector, 6000, ui.ObjEventTypeAppear).Snapshot(takeSnapshot).Do(ctx)

	if !isStop {
		clickShutterButton(ctx, client, mutDevice)
	}

	openCapturedFrameOrRecordedVideo(ctx, client, mutDevice)

	videoFileName, mtbferror := clickInfoAndGetFileName(ctx, client, mutDevice)
	if mtbferror != nil {
		return "", mtbferror
	}

	pressCancelButton(ctx, client, mutDevice, 2)

	return videoFileName, nil
}

func startAudiorecorder(ctx context.Context, client sdk.DelegateClient, mutDevice string) error {

	for i := 0; i < 3; i++ {
		client.StartMainActivity(mutDevice, defaultMainAct, barPkg).Do(ctx, service.Sleep(time.Second*2))

		client.UIAClick(mutDevice).Selector(closeAppSelector).Do(ctx, service.Suppress())

		isNoThank, _ := client.UIAObjEventWait(mutDevice,
			barNoThanksSelector, 6000, ui.ObjEventTypeAppear).Snapshot(takeSnapshot).Do(ctx)

		if !isNoThank {
			pressCancelButton(ctx, client, mutDevice, 1)
		}

		client.UIAClick(mutDevice).Selector(noSelector).Snapshot(takeSnapshot).Do(ctx, service.Suppress())
		client.UIAClick(mutDevice).Selector(containsNoSelector).Snapshot(takeSnapshot).Do(ctx, service.Suppress())

		// Allow the permission
		allowPermission(ctx, client, mutDevice, permissionAllowSelector)

		// Verify whether enter to correct app.
		isEnterApp, _ := client.UIAObjEventWait(mutDevice,
			barVoiceRecorderSelector, 6000, ui.ObjEventTypeAppear,
		).FailOnNotMatch(notFailOnNotMatch).Snapshot(takeSnapshot).Do(ctx)

		if isEnterApp {
			break
		}
		if !isEnterApp && i >= 2 {
			return mtbferrors.New(mtbferrors.VoiceRecordApp, nil)
		}
	}
	return nil
}

func recordSound(ctx context.Context, client sdk.DelegateClient) (string, error) {
	mtbferr := startAudiorecorder(ctx, client, mutDevice)
	if mtbferr != nil {
		return "", mtbferr
	}
	// Start record audio
	client.UIAClick(mutDevice).Selector(barRecordStartSelector).Snapshot(takeSnapshot).Do(
		ctx, service.Sleep(time.Second*25))

	isStarted, _ := client.UIAObjEventWait(mutDevice,
		barPauseStartSelector, 6000, ui.ObjEventTypeAppear).Do(ctx)

	if !isStarted {
		return "", mtbferrors.New(mtbferrors.StartRecord, nil)
	}
	client.UIAClick(mutDevice).Selector(barRecordStartSelector).Snapshot(takeSnapshot).Do(ctx)

	audioFileName, mtbferror := getAudioFileName(ctx, client, mutDevice)
	if mtbferror != nil {
		return "", mtbferror
	}

	client.UIAClick(mutDevice).Selector(okSelector).Do(ctx, service.Suppress())
	return audioFileName, nil
}

func allowPermission(ctx context.Context, client sdk.DelegateClient, mutDevice, selector string) {
	isNotInit, _ := client.UIAObjEventWait(mutDevice,
		selector, 2000, ui.ObjEventTypeAppear).Do(ctx)

	if isNotInit {
		for i := 0; i < 5; i++ {
			client.UIAClick(mutDevice).Selector(selector).Do(ctx)

			isNotInit, _ := client.UIAObjEventWait(mutDevice,
				selector, 3000, ui.ObjEventTypeAppear).Do(ctx)

			if !isNotInit {
				break
			}
		}
	}
}

func getAudioFileName(ctx context.Context, client sdk.DelegateClient, deviceID string) (string, error) {
	name, _ := client.GetWidgetText2(deviceID, barEdtFileNameSelector).Do(ctx)
	if len(name) == 0 {
		return "", mtbferrors.New(mtbferrors.CannotGetRecordedFile, nil)
	}

	client.Comments(fmt.Sprintf("Got an file: %s", name)).Do(ctx)
	return name + ".mp3", nil
}

func deleteCreatedFrameOrVideo(ctx context.Context, client sdk.DelegateClient, deviceID string, times int) {
	for i := 0; i < times; i++ {
		client.UIAClick(deviceID).Selector(gcaDeleteSelector).Do(ctx)
	}
}

func deleteCreatedAudioFile(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	// Click to review all audio list
	client.UIAClick(deviceID).Selector(barTabFileListSelector).Snapshot(takeSnapshot).Do(ctx)

	// Click delete button
	isDelete, _ := client.UIAObjEventWait(deviceID,
		barDelFileListSelector, 3000, ui.ObjEventTypeAppear).Do(ctx)

	if !isDelete {
		pressCancelButton(ctx, client, mutDevice, 1)
	}

	client.UIAClick(deviceID).Selector(barDelFileListSelector).Do(ctx)

	// Select created audio file
	for i := 0; i < 3; i++ {
		client.UIAClickListItem(deviceID,
			fmt.Sprintf("class=android.widget.FrameLayout::index=%d", i)).ScrollSelector(
			barFileListSelector).Snapshot(takeSnapshot).Do(ctx)
	}
	// Delete the audio file
	client.UIAClick(deviceID).Selector(delSelector).Do(ctx)

	client.UIAClick(deviceID).Selector(okSelector).Do(ctx)
}

func clickGCAThumbnail(ctx context.Context, client sdk.DelegateClient, deviceID string) {
	client.UIAClick(deviceID).Selector(gcaThumbnailSelector).Snapshot(takeSnapshot).Do(ctx)
}
