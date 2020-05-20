// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cienet.com/cats/node"
	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	uiSvc "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF066MediaFilesInFilesAPP,
		Desc:     "Android media files will be under Chrome Files App",
		Contacts: []string{"roger.cheng@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"cats.requestURL",
		},
		Timeout:      15 * time.Minute,
		SoftwareDeps: []string{"chrome", "chrome_internal", "android"},
		ServiceDeps: []string{
			"tast.mtbf.ui.FilesAppService",
			"tast.mtbf.svc.CommService"},
	})
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

// MTBF066MediaFilesInFilesAPP Run MTBF066 case
func MTBF066MediaFilesInFilesAPP(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF066MediaFilesInFilesAPP",
		Description: "Android media files will be under Chrome Files App",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		// New GRPC connection
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		// New GRPC Client
		cr := uiSvc.NewFilesAppServiceClient(cl.Conn)

		caseResult, mtbferror := runMTBF066MainStage(ctx, client)

		if mtbferror != nil {
			s.Fatal(mtbferror)
		}

		if caseResult == nil {
			s.Fatal(mtbferrors.New(mtbferrors.ARCParseResult, errors.New("CATS case result is nil")))
		}

		s.Logf("CATS Node Case caseResult: %+v", caseResult)

		result, ok := caseResult.(mtbf066MainStageResult)
		if !ok {
			s.Fatal(mtbferrors.New(mtbferrors.ARCParseResult, nil))
		}

		downloadFiles, err := cr.GetAllFiles(ctx, &uiSvc.FoldersRequest{Folders: []string{"Downloads"}})
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

		audioFiles, err := cr.GetAllFiles(ctx, &uiSvc.FoldersRequest{Folders: []string{"Audio"}})
		if err != nil {
			s.Fatal(mtbferrors.NewGRPCErr(err))
		}
		s.Logf("Audio files: %#v", audioFiles.GetFiles())

		if !contains(audioFiles.GetFiles(), result.Audio) {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCFileNotFound, nil, result.Audio))
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
	logLevel                  = node.LogLevel(6)
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

// mtbf066MainStageResult holds those miedia file names.
type mtbf066MainStageResult struct {
	Photo string `json:"photo"`
	Video string `json:"video"`
	Audio string `json:"audio"`
}

// runMTBF066MainStage runs main stage
func runMTBF066MainStage(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
	// 1. Use an Android camera app to take photos and videos.
	// 2. Use an Android recorder app to take a sound clip.
	imageFileName, mtbferror := takePhoto(ctx, client, mutDevice)
	if mtbferror != nil {
		return nil, mtbferror
	}

	videoFileName, mtbferror := recordVideo(ctx, client, mutDevice)
	if mtbferror != nil {
		return nil, mtbferror
	}

	audioFileName, mtbferror := recordSound(ctx, client, mutDevice)
	if mtbferror != nil {
		return nil, mtbferror
	}

	return mtbf066MainStageResult{
		Photo: imageFileName,
		Video: videoFileName,
		Audio: audioFileName,
	}, nil
}

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

func takePhoto(ctx context.Context, client sdk.DelegateClient, deviceID string) (string, error) {
	// global image_file_name
	client.StartMainActivity(mutDevice, gcaAct, gcaPkg).Do(ctx, service.Sleep(time.Second*2))

	// If App crash, close the dialogue window
	client.UIAClick(deviceID).Selector(closeAppSelector).Do(ctx, service.Suppress())

	// Click OK
	client.UIAClick(deviceID).Selector(okSelector).Do(ctx, service.Suppress())

	// Verify whether enter to Camera app
	isEnterApp, _ := client.UIAObjEventWait(deviceID,
		gcaSelector, 6000, ui.ObjEventTypeAppear).FailOnNotMatch(
		notFailOnNotMatch).Snapshot(takeSnapshot).Do(ctx)

	if !(isEnterApp) {
		return "", mtbferrors.New(mtbferrors.EnterCameraApp, nil)
	}

	// Delete all formrt created images or videos
	deleteAllImagesOrVideos(ctx, client, deviceID)

	// Verify whethr in taking photo page. If not, Switch to photo page.
	isPhoto, _ := client.UIAObjEventWait(deviceID,
		gcaShutterSelector, 6000, ui.ObjEventTypeAppear).Do(ctx)

	if !(isPhoto) {
		client.UIAClick(deviceID).Selector(gcaPhotoSwitchSelector).Snapshot(takeSnapshot).Do(ctx)
	}
	// Take photo
	clickShutterButton(ctx, client, deviceID)

	// Open captured frame to check.
	openCapturedFrameOrRecordedVideo(ctx, client, deviceID)

	// Click info button and get file name
	imageFileName, mtbferror := clickInfoAndGetFileName(ctx, client, deviceID)
	if mtbferror != nil {
		return "", mtbferror
	}

	// Exit to main page of Camera app
	pressCancelButton(ctx, client, deviceID, 2)
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
	// Click info button.
	client.UIAClick(deviceID).Selector(gcaDetailsSelector).Do(ctx)

	rsp, _ := client.GetWidgetText2(deviceID, gcaTitleSelector).Do(ctx)

	if len(rsp) > 0 {
		imageFile := (rsp)[7:]
		if strings.HasPrefix(imageFile, "IMG_") {
			imageFile = fmt.Sprintf("%s.jpg", imageFile)
		}
		client.Comments(fmt.Sprintf("Got an file: %s", imageFile)).Do(ctx)
		return imageFile, nil
	}
	return "", mtbferrors.New(mtbferrors.CanootGetImgOrVideoFileName, nil)
}

func pressCancelButton(ctx context.Context, client sdk.DelegateClient, deviceID string, times int32) {
	client.Press(deviceID, ui.OprKeyEventCANCEL).Times(times).Do(ctx)
}

func recordVideo(ctx context.Context, client sdk.DelegateClient, deviceID string) (string, error) {
	isVideo, _ := client.UIAObjEventWait(deviceID,
		gcaStartRecordingSelector, 6000, ui.ObjEventTypeAppear).Do(ctx)

	if !isVideo {
		client.UIAClick(deviceID).Selector(gcaVideoSwitchSelector).Snapshot(takeSnapshot).Do(ctx, service.Sleep(time.Second*2))
	}
	clickShutterButton(ctx, client, deviceID)

	client.Delay(5000).Do(ctx)

	clickShutterButton(ctx, client, deviceID)

	isStop, _ := client.UIAObjEventWait(deviceID,
		gcaStartRecordingSelector, 6000, ui.ObjEventTypeAppear).Snapshot(takeSnapshot).Do(ctx)

	if !isStop {
		clickShutterButton(ctx, client, deviceID)
	}

	openCapturedFrameOrRecordedVideo(ctx, client, mutDevice)

	videoFileName, mtbferror := clickInfoAndGetFileName(ctx, client, mutDevice)
	if mtbferror != nil {
		return "", mtbferror
	}

	pressCancelButton(ctx, client, deviceID, 2)

	return videoFileName, nil
}

func startAudiorecorder(ctx context.Context, client sdk.DelegateClient, deviceID string) error {

	for i := 0; i < 3; i++ {
		client.StartMainActivity(deviceID, defaultMainAct, barPkg).Do(ctx, service.Sleep(time.Second*2))

		client.UIAClick(deviceID).Selector(closeAppSelector).Do(ctx, service.Suppress())

		isNoThank, _ := client.UIAObjEventWait(deviceID,
			barNoThanksSelector, 6000, ui.ObjEventTypeAppear).Snapshot(takeSnapshot).Do(ctx)

		if !isNoThank {
			pressCancelButton(ctx, client, deviceID, 1)
		}

		client.UIAClick(deviceID).Selector(noSelector).Snapshot(takeSnapshot).Do(ctx, service.Suppress())
		client.UIAClick(deviceID).Selector(containsNoSelector).Snapshot(takeSnapshot).Do(ctx, service.Suppress())

		// Allow the permission
		allowPermission(ctx, client, deviceID, permissionAllowSelector)

		// Verify whether enter to correct app.
		isEnterApp, _ := client.UIAObjEventWait(deviceID,
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

func recordSound(ctx context.Context, client sdk.DelegateClient, deviceID string) (string, error) {
	mtbferr := startAudiorecorder(ctx, client, deviceID)
	if mtbferr != nil {
		return "", mtbferr
	}
	// Start record audio
	client.UIAClick(deviceID).Selector(barRecordStartSelector).Snapshot(takeSnapshot).Do(
		ctx, service.Sleep(time.Second*25))

	isStarted, _ := client.UIAObjEventWait(deviceID,
		barPauseStartSelector, 6000, ui.ObjEventTypeAppear).Do(ctx)

	if !isStarted {
		return "", mtbferrors.New(mtbferrors.StartRecord, nil)
	}
	client.UIAClick(deviceID).Selector(barRecordStartSelector).Snapshot(takeSnapshot).Do(ctx)

	audioFileName, mtbferror := getAudioFileName(ctx, client, deviceID)
	if mtbferror != nil {
		return "", mtbferror
	}

	client.UIAClick(deviceID).Selector(okSelector).Do(ctx, service.Suppress())
	return audioFileName, nil
}

func allowPermission(ctx context.Context, client sdk.DelegateClient, deviceID string, selector string) {
	isNotInit, _ := client.UIAObjEventWait(deviceID,
		selector, 2000, ui.ObjEventTypeAppear).Do(ctx)

	if isNotInit {
		for i := 0; i < 5; i++ {
			client.UIAClick(deviceID).Selector(selector).Do(ctx)

			isNotInit, _ := client.UIAObjEventWait(deviceID,
				selector, 3000, ui.ObjEventTypeAppear).Do(ctx)

			if !isNotInit {
				break
			}
		}
	}
}

func getAudioFileName(ctx context.Context, client sdk.DelegateClient, deviceID string) (string, error) {
	// Get the audio file name
	rsp, _ := client.GetWidgetText2(deviceID, barEdtFileNameSelector).Do(ctx)

	var audioFileName string
	if len(rsp) > 0 {
		audioFileName = rsp
		client.Comments(fmt.Sprintf("Got an file: %s", audioFileName)).Do(ctx)
	} else {
		return "", mtbferrors.New(mtbferrors.CannotGetRecordedFile, nil)
	}
	return audioFileName, nil
}

func deleteCreatedFrameOrVideo(ctx context.Context, client sdk.DelegateClient, deviceID string, times int) {
	for i := 0; i < times; i++ {
		client.UIAClick(deviceID).Selector(gcaDeleteSelector).Do(ctx)

		client.Press(deviceID, ui.OprKeyEventCANCEL).Times(1).Do(ctx)
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
