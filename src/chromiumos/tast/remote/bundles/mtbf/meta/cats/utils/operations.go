// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cienet.com/cats/node/sdk/basic"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/testing"
)

// AllowPermission allows the permission when it pops up on DUT.
func (dev *Device) AllowPermission(ctx context.Context, selector string) error {
	notInit, _ := dev.Client.UIAObjEventWait(dev.DeviceID, selector, 2000, ui.ObjEventTypeAppear).Do(ctx)

	if notInit {
		for i := 0; i < 5; i++ {
			dev.Client.UIAClick(dev.DeviceID).Selector(selector).Do(ctx)
			notInit, _ := dev.Client.UIAObjEventWait(dev.DeviceID, selector, 3000, ui.ObjEventTypeAppear).Do(ctx)
			if !notInit {
				break
			}
		}
	}

	return nil
}

// ClickSelector finds selector and click it if it exists.
func (dev *Device) ClickSelector(ctx context.Context, selector string) error {
	if isExist, err := dev.Client.UIAObjEventWait(dev.DeviceID, selector, 1000, ui.ObjEventTypeAppear).Do(ctx); err != nil {
		return err
	} else if isExist {
		testing.ContextLogf(ctx, "Click Selector %q", selector)
		dev.Client.UIAClick(dev.DeviceID).Selector(selector).Do(ctx)
	} else {
		testing.ContextLogf(ctx, "Selector %q not found", selector)
	}
	return nil
}

// UnlockPhone unlockes the DUT.
func (dev *Device) UnlockPhone(ctx context.Context) {
	dev.Client.Press(dev.DeviceID, ui.OprKeyEventHOME).Times(2).Do(ctx)
	dev.Client.Press(dev.DeviceID, ui.OprKeyEventCANCEL).Times(2).Do(ctx)
	dev.Client.Press(dev.DeviceID, ui.OprKeyEventMENU).Do(ctx)
}

// JoinAppRtcCleanup does cleanup for Join RTC App.
func (dev *Device) JoinAppRtcCleanup(ctx context.Context) {
	dev.Client.Press(dev.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	dev.Client.Press(dev.DeviceID, ui.OprKeyEventHOME).Do(ctx)
}

// PressCancelButton presses DUT CANCEL button.
func (dev *Device) PressCancelButton(ctx context.Context, times int) error {
	return dev.Client.Press(dev.DeviceID, ui.OprKeyEventCANCEL).Times(int32(times)).Do(ctx)
}

// ClickCloseButton clicks DUT close button.
func (dev *Device) ClickCloseButton(ctx context.Context) {
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=android:id/decor_close_button").Do(ctx)
}

// ClickInfoAndGetFilename clicks "Details" and gets file name.
func (dev *Device) ClickInfoAndGetFilename(ctx context.Context) (string, error) {
	dev.Client.UIAClick(dev.DeviceID).Selector("desc=Details").Do(ctx)
	name, _ := dev.Client.GetWidgetText2(dev.DeviceID, "ID=android:id/text1::textstartwith=Title:").Do(ctx)

	if strings.HasPrefix(name, "IMG") {
		name += ".jpg"
	}

	return name, nil
}

// ClickShutterButton clicks the shutter button.
func (dev *Device) ClickShutterButton(ctx context.Context) {
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/shutter_button").Do(ctx)
}

// DeleteAllImagesOrVideos deletes all images or videos.
func (dev *Device) DeleteAllImagesOrVideos(ctx context.Context) {
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/thumbnail_button").Do(ctx)

	enter, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete::desc=Delete", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if enter {
		for {
			noFile, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "text=You have no photos", 1000, ui.ObjEventTypeAppear).Do(ctx)
			if noFile {
				break
			}
			dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete").Do(ctx)
		}
		dev.Client.Press(dev.DeviceID, ui.OprKeyEventCANCEL).Do(ctx)
	} else {
		dev.Client.Comments("No any image or video in gallery.").Do(ctx)
	}
}

// DeleteCreatedFrameOrVideo deleteds created frame or video.
func (dev *Device) DeleteCreatedFrameOrVideo(ctx context.Context, times int) {
	for i := 0; i < times; i++ {
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete").Do(ctx)
	}

	dev.Client.Press(dev.DeviceID, ui.OprKeyEventCANCEL).Do(ctx)
}

// WaitForGCAScreen waits for GCA screen.
func (dev *Device) WaitForGCAScreen(ctx context.Context, evt ui.ObjEventType) error {
	enter, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "packagename=com.google.android.GoogleCameraArc", 12000, evt).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.GCACamera, nil)
	}

	return nil
}

// EnterToConversationAndSelectVideo enters the conversation and selects video.
func (dev *Device) EnterToConversationAndSelectVideo(ctx context.Context) error {
	conv, _ := dev.Client.UIAVerifyListItem(dev.DeviceID,
		"ID=com.google.android.talk:id/conversationContent").ParentSelector(
		"class=android.widget.LinearLayout::index=0").ScrollSelector(
		"ID=android:id/list").Do(ctx)

	if !conv {
		return mtbferrors.New(mtbferrors.NoConversation, nil)
	}

	dev.Client.UIAClickListItem(dev.DeviceID,
		"ID=com.google.android.talk:id/conversationContent").ParentSelector(
		"class=android.widget.LinearLayout::index=0").ScrollSelector(
		"ID=android:id/list").Do(ctx)

	enter, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.talk:id/message_text", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.EnterConversationPage, nil)
	}

	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.talk:id/video_camerapicker_indicator_icon").Do(ctx)
	dev.AllowPermission(ctx, "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button")
	return dev.WaitForGCAScreen(ctx, ui.ObjEventTypeAppear)
}

// SendMessageToChromeOS sends message to ChromeOS.
func (dev *Device) SendMessageToChromeOS(ctx context.Context, message string) error {
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.talk:id/message_text").Do(ctx)
	dev.InputText(ctx, message)
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.talk:id/floating_send_button").Do(ctx)

	sent, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.talk:id/footer_text::textcontains=Now", 60000, ui.ObjEventTypeAppear).Do(ctx)
	if !sent {
		return mtbferrors.New(mtbferrors.SendHangoutsMessage, nil)
	}

	return nil
}

// EnterToConversation enters conversation.
func (dev *Device) EnterToConversation(ctx context.Context, contact string) error {
	dev.Client.UIAClick(dev.DeviceID).Selector("desccontains=" + contact).Do(ctx)

	enter, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "textcontains="+contact, 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.EnterContactPage, nil, contact)
	}

	return nil
}

// DeleteCreatedAudioFile deletes created audio file.
func (dev *Device) DeleteCreatedAudioFile(ctx context.Context) error {
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/tab_file_list").Do(ctx)
	delete, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.media.bestrecorder.audiorecorder:id/btn_delete_list", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if !delete {
		dev.PressCancelButton(ctx, 1)
	}

	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_delete_list").Do(ctx)

	for i := 0; i < 3; i++ {
		target := fmt.Sprintf("class=android.widget.FrameLayout::index=%d", i)
		scroll := fmt.Sprintf("ID=com.media.bestrecorder.audiorecorder:id/listview_file")
		dev.Client.UIAClickListItem(dev.DeviceID, target).ScrollSelector(scroll).Do(ctx)
	}

	dev.Client.UIAClick(dev.DeviceID).Selector("text=Delete").Do(ctx)
	dev.Client.UIAClick(dev.DeviceID).Selector("text=OK").Do(ctx)

	return nil
}

// EnterToAppAndVerify enters APP and does verification.
func (dev *Device) EnterToAppAndVerify(ctx context.Context, act, pkg, selector string) error {
	dev.Client.StartMainActivity(dev.DeviceID, act, pkg).Do(ctx)

	close, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "text=Close app", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if close {
		dev.Client.UIAClick(dev.DeviceID).Selector("text=Close app").Do(ctx)
	}

	enter, _ := dev.Client.UIAObjEventWait(dev.DeviceID, selector, 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.EnterAppMainPage, nil, pkg)
	}

	return nil
}

// GetIP gets IP address.
func (dev *Device) GetIP(ctx context.Context) string {
	connectID, _ := dev.Client.GetDutConfigItem(dev.DeviceID, basic.DutConfigKeyConnectID).Do(ctx)
	ip := strings.Split(connectID, ":")[0]
	dev.Client.Comments(ip).Do(ctx)
	return ip
}

// GetDeviceYaml gets device yaml.
func (dev *Device) GetDeviceYaml(ctx context.Context) string {
	yaml, _ := dev.Client.GetDutConfigItem(dev.DeviceID, basic.DutConfigKeyDeviceName).Do(ctx)
	dev.Client.Comments(yaml).Do(ctx)
	return yaml
}

// GetAudioFileName gets  audio file name.
func (dev *Device) GetAudioFileName(ctx context.Context) (string, error) {
	name, _ := dev.Client.GetWidgetText2(dev.DeviceID, "id=com.media.bestrecorder.audiorecorder:id/edt_file_name").Do(ctx)

	if name == "" {
		return name, mtbferrors.New(mtbferrors.CannotGetRecordedFile, nil)
	}

	return name, nil
}

// GetCurrentPlayingTimeOfVideo gets current playing time of a video.
func (dev *Device) GetCurrentPlayingTimeOfVideo(ctx context.Context, coordinate, resourceID string) string {
	dev.Client.UIAClick(dev.DeviceID).Coordinate(coordinate).Snapshot(false).Do(ctx)
	t, _ := dev.Client.GetWidgetText2(dev.DeviceID, "ID="+resourceID).Do(ctx)
	return t
}

// OpenCapturedFrameOrRecordedVideo opens captured frame or recorded video.
func (dev *Device) OpenCapturedFrameOrRecordedVideo(ctx context.Context) error {
	dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/thumbnail_button").Do(ctx)

	enter, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.GoogleCameraArc:id/filmstrip_bottom_control_delete::desc=Delete", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.EnterGallery, nil)
	}

	return nil
}

// OpenGoogleMusicAndPlay opens Google Music and plays music.
func (dev *Device) OpenGoogleMusicAndPlay(ctx context.Context) error {
	dev.EnterToAppAndVerify(ctx, "com.android.music.activitymanagement.TopLevelActivity", "com.google.android.music", "packagename=com.google.android.music")
	dev.ClickSelector(ctx, "text=NO THANKS")
	dev.ClickSelector(ctx, "textstartwith=SKIP")
	dev.ClickSelector(ctx, "descstartwith=SKIP")
	dev.ClickSelector(ctx, "text=GOT IT")

	lib, _ := dev.Client.UIASearchElement(dev.DeviceID,
		"text=Music library").ReferenceSelector(
		"desc=Show navigation drawer").ParentSelector(
		"ID=com.google.android.music:id/play_music_header_toolbar").Do(ctx)

	found := lib.Found

	if !found {
		dev.ClickSelector(ctx, "desc=Show navigation drawer")
		dev.ClickSelector(ctx, "text=Music library")
	}
	dev.ClickSelector(ctx, "text=ALBUMS")

	res, _ := dev.Client.UIAObjEventWait(dev.DeviceID,
		"text=A Song For You", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if !res {
		return mtbferrors.New(mtbferrors.CannotFindTargetFile, nil)
	}

	play, _ := dev.Client.UIAObjEventWait(dev.DeviceID,
		"ID=com.google.android.music:id/li_play_button::desc=Pause A Song For You",
		2000, ui.ObjEventTypeAppear).Do(ctx)
	if !play {
		dev.Client.UIAClick(dev.DeviceID).Selector(
			"ID=com.google.android.music:id/li_play_button::desc=Play A Song For You").Do(ctx)
	}

	play, _ = dev.Client.UIAObjEventWait(dev.DeviceID,
		"id=com.google.android.music:id/pause::desc=Pause", 8000, ui.ObjEventTypeAppear,
	).FailOnNotMatch(true).Do(ctx)
	if !play {
		return mtbferrors.New(mtbferrors.GoogleMusicNotPlay, nil)
	}

	return nil
}

// VerifyAndSkipAd verifies and skips ad.
func (dev *Device) VerifyAndSkipAd(ctx context.Context) {
	ad, _ := dev.Client.UIAObjEventWait(dev.DeviceID,
		"id=com.google.android.youtube:id/ad_progress_text::textstartwith=Ad", 6000, ui.ObjEventTypeAppear,
	).Snapshot(false).Do(ctx)
	if ad {
		//dev.Client.UIAClick(dev.DeviceID).Selector("id=com.google.android.youtube:id/skip_ad_button_text::textcontains=Skip ad"), UIAClickNoSnapshot(), UIAClickServiceOptions(service.Suppress()).Do(ctx)
		dev.ClickSelector(ctx, "id=com.google.android.youtube:id/skip_ad_button_text::textcontains=Skip ad")
	}

	ad, _ = dev.Client.UIAObjEventWait(dev.DeviceID,
		"id=com.google.android.youtube:id/ad_progress_text::textstartwith=Ad", 6000, ui.ObjEventTypeAppear,
	).Snapshot(false).Do(ctx)
	if ad {
		dev.Client.Delay(35000).Do(ctx)
	}
}

// SendHangoutsMessage sends hangout messages.
func (dev *Device) SendHangoutsMessage(ctx context.Context, contact string) error {
	testing.ContextLog(ctx, "Start sending Hangouts message")
	dev.Client.StartMainActivity(
		dev.DeviceID,
		".SigningInActivity",
		"com.google.android.talk").Do(ctx, service.Sleep(time.Second*2))
	isNext, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "text=SKIP::ID=com.google.android.talk:id/promo_button_no", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if isNext {
		dev.Client.UIAClick(dev.DeviceID).Selector("text=SKIP::ID=com.google.android.talk:id/promo_button_no").Do(ctx)
	}

	dev.AllowPermission(ctx, "ID=com.android.permissioncontroller:id/permission_allow_button")
	testing.ContextLog(ctx, "Enter Hangouts Talk app")
	enterApp, _ := dev.Client.UIAObjEventWait(dev.DeviceID,
		"ID=com.google.android.talk:id/title::text=Hangouts", 6000, ui.ObjEventTypeAppear,
	).FailOnNotMatch(true).Do(ctx)
	if !enterApp {
		return mtbferrors.New(mtbferrors.CompHangoutsApp, nil)
	}

	testing.ContextLog(ctx, "Enter to conversation")
	dev.EnterToConversation(ctx, contact)

	testing.ContextLog(ctx, "Send Hangouts message")
	dev.SendMessageToChromeOS(ctx, "Hi")
	return nil
}

// OpenVLCAndEnterToDownload opens vlc player and enters download folder.
func (dev *Device) OpenVLCAndEnterToDownload(ctx context.Context) error {
	// Open VLC player and goto directories option.
	dev.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")
	// Initialize VLC
	isNext, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=org.videolan.vlc:id/next", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if isNext {
		// Click 'Next', 'Allow', 'Next', 'Done'
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=org.videolan.vlc:id/next").Do(ctx)
		isNotInit, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button", 4000, ui.ObjEventTypeAppear).Do(ctx)
		if isNotInit {
			dev.Client.UIAClick(dev.DeviceID).Selector("text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button").Do(ctx)
		}
		isNext, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=org.videolan.vlc:id/next", 3000, ui.ObjEventTypeAppear).Do(ctx)
		if isNext {
			dev.Client.UIAClick(dev.DeviceID).Selector("ID=org.videolan.vlc:id/next").Do(ctx)
			dev.Client.UIAClick(dev.DeviceID).Selector("ID=org.videolan.vlc:id/doneButton").Do(ctx)
			dev.ClickSelector(ctx, "ID=org.videolan.vlc:id/design_menu_item_text::text=Directories")
		}
	}
	dev.ClickSelector(ctx, "textstartwith=Got it")
	// Verify if in "Directories" tab. If not, open navigation drawer and click "Directories"
	lib, _ := dev.Client.UIASearchElement(dev.DeviceID, "text=Directories").ReferenceSelector(
		"desc=Open navigation drawer").Do(ctx)
	found := lib.Found
	if !found {
		dev.Client.UIAClick(dev.DeviceID).Selector("desc=Open navigation drawer").Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=org.videolan.vlc:id/design_menu_item_text::text=Directories").Do(ctx)
	}
	dev.ClickSelector(ctx, "ID=org.videolan.vlc:id/title::text=Download")
	sent, _ := dev.Client.UIAVerify(dev.DeviceID, "ID=org.videolan.vlc:id/title::text=audios").Do(ctx)
	if !sent.True {
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=Download").Do(ctx)
	}
	return nil
}

// OpenSpotifyAndPlay opens spotify and plays music.
func (dev *Device) OpenSpotifyAndPlay(ctx context.Context) error {
	dev.EnterToAppAndVerify(ctx, ".MainActivity", "com.spotify.music", "packagename=com.spotify.music")
	dev.ClickSelector(ctx, "descstartwith=NO")
	dev.ClickSelector(ctx, "textstartwith=NO")
	dev.ClickSelector(ctx, "text=CLOSE")
	dev.ClickSelector(ctx, "desc=CLOSE")

	const (
		spotifyIDPrefix   = "ID=com.spotify.music:id/"
		playPauseBtnSel   = spotifyIDPrefix + "play_pause_button"
		searchNavItemSel  = spotifyIDPrefix + "bottom_navigation_item_icon::desc=Search"
		searchFieldSel    = spotifyIDPrefix + "find_search_field"
		shufflePlayBtnSel = spotifyIDPrefix + "children::class=android.widget.LinearLayout"
		recycleViewSel    = "class=androidx.recyclerview.widget.RecyclerView"
		albumName         = "Lover"
	)
	testing.ContextLog(ctx, "Check if the Play button exists")
	init, _ := dev.Client.UIAObjEventWait(dev.DeviceID, playPauseBtnSel, 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !init {
		testing.ContextLog(ctx, "The Play button doesn't exists")
		dev.Client.UIAClick(dev.DeviceID).Selector(searchNavItemSel).Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector(searchFieldSel).Do(ctx)
		dev.InputText(ctx, albumName)
		dev.Client.UIAClickListItem(dev.DeviceID, "ID=android:id/text1::text="+albumName).
			ReferenceSelector("ID=android:id/text2::textstartwith=Album").
			ScrollSelector(recycleViewSel).Do(ctx)
		testing.ContextLog(ctx, "Click the button 'SHUFFLE PLAY'")
		dev.ClickSelector(ctx, "text=SHUFFLE PLAY")
		dev.ClickSelector(ctx, shufflePlayBtnSel)
	} else {
		testing.ContextLog(ctx, "Check if it is playing")
		play, _ := dev.Client.UIAObjEventWait(dev.DeviceID, playPauseBtnSel+"::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !play {
			testing.ContextLog(ctx, "Click the Play button")
			dev.Client.UIAClick(dev.DeviceID).Selector(playPauseBtnSel).Do(ctx)
		}
	}

	testing.ContextLog(ctx, "Verify whether it is playing")
	play, _ := dev.Client.UIAObjEventWait(dev.DeviceID, playPauseBtnSel+"::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
	testing.ContextLog(ctx, "isPlaying = ", play)
	if !play {
		return mtbferrors.New(mtbferrors.SpotifyNotPlay, nil)
	}

	return nil
}

// OpenYoutubeMusicAndPlay opens youtube music and plays music.
func (dev *Device) OpenYoutubeMusicAndPlay(ctx context.Context) error {
	init := false
	dev.EnterToAppAndVerify(ctx, ".activities.MusicActivity", "com.google.android.apps.youtube.music", "packagename=com.google.android.apps.youtube.music")
	//dev.Client.UIAClick(dev.DeviceID).Selector("text=DISMISS"), UIAClickServiceOptions(service.Suppress()).Do(ctx)
	dev.ClickSelector(ctx, "text=DISMISS")
	//dev.Client.UIAClick(dev.DeviceID).Selector("textstartwith=SKIP"), UIAClickServiceOptions(service.Suppress()).Do(ctx)
	dev.ClickSelector(ctx, "textstartwith=SKIP")
	//dev.Client.UIAClick(dev.DeviceID).Selector("descstartwith=SKIP"), UIAClickServiceOptions(service.Suppress()).Do(ctx)
	dev.ClickSelector(ctx, "descstartwith=SKIP")
	//dev.Client.UIAClick(dev.DeviceID).Selector("text=NO, THANKS"), UIAClickServiceOptions(service.Suppress()).Do(ctx)
	dev.ClickSelector(ctx, "text=NO, THANKS")
	//dev.Client.UIAClick(dev.DeviceID).Selector("desc=Close"), UIAClickServiceOptions(service.Suppress()).Do(ctx)
	dev.ClickSelector(ctx, "desc=Close")

	list, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.apps.youtube.music:id/play_pause_replay_button", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !list {
		init = true
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/action_search_button").Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/search_edit_text").Do(ctx)
		dev.InputText(ctx, "Taylor Swift")
		dev.Client.Press(dev.DeviceID, ui.OprKeyEventENTER).Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/chip_cloud_chip_text::text=Artists").Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/title::text=Taylor Swift").Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/play_specialty_button::text=SHUFFLE").Do(ctx)

		play, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.apps.youtube.music:id/player_control_play_pause_replay_button::desc=Pause video", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !play {
			return mtbferrors.New(mtbferrors.YoutubeMusicNotPlay, nil)
		}
	} else {
		play, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.apps.youtube.music:id/play_pause_replay_button::desc=Pause video", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !play {
			dev.Client.UIAClick(dev.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/play_pause_replay_button").Do(ctx)
		}

		play, _ = dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.apps.youtube.music:id/play_pause_replay_button::desc=Pause video", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !play {
			return mtbferrors.New(mtbferrors.YoutubeMusicNotPlay, nil)
		}
	}

	if init {
		dev.PressCancelButton(ctx, 1)
	}

	return nil
}

// RecordVideoAndPlay records video and plays it.
func (dev *Device) RecordVideoAndPlay(ctx context.Context) error {
	dev.ClickShutterButton(ctx)

	record, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Stop Recording", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !record {
		return mtbferrors.New(mtbferrors.StartRecordVideo, nil)
	}

	dev.ClickShutterButton(ctx)
	dev.Client.UIAClick(dev.DeviceID).Selector("desc=Review").Do(ctx)

	open, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "text=Open with", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if open {
		dev.Client.UIAClick(dev.DeviceID).Selector("text=VLC").Do(ctx)
		dev.Client.UIAClick(dev.DeviceID).Selector("text=ALWAYS").Do(ctx)
	}

	vlc, _ := dev.Client.UIAObjEventWait(dev.DeviceID, "packagename=org.videolan.vlc", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if vlc {
		dev.Client.ExecCommand(dev.DeviceID, "shell am force-stop org.videolan.vlc").Do(ctx)
	}

	dev.Client.UIAObjEventWait(dev.DeviceID, "desc=Retake", 20000, ui.ObjEventTypeAppear).Do(ctx)
	return nil
}

// GetCoordinateForTargetApp gets coordinate for target APP.
func (dev *Device) GetCoordinateForTargetApp(ctx context.Context, appName string, coordMap map[string]string) (string, error) {
	wmsize, _ := dev.Client.ExecAdbCommand(dev.DeviceID, "shell wm size").Do(ctx)
	width, height := 0, 0
	_, err := fmt.Sscanf(wmsize.StandardOutput, "Physical: %dx%d", &width, &height)
	if err != nil {
		return "", mtbferrors.New(mtbferrors.GetDUTResolution, nil)
	}

	resName := fmt.Sprintf("%s.%d", appName, width)
	coord, ok := coordMap[resName]

	if !ok {
		resName = fmt.Sprintf("%s.default", appName)
		coord, ok = coordMap[resName]
		if !ok {
			return "", mtbferrors.New(mtbferrors.LostResolutionConfig, nil)
		}
	}

	dev.Client.Comments(
		fmt.Sprintf("Width: %d, App name: %s, Coordinate: %s.", width, appName, coord)).Do(ctx)

	return coord, nil
}

// ClickCoordinate clicks at coord of the device without snapshot.
func (dev *Device) ClickCoordinate(ctx context.Context, coord string) {
	dev.Client.Click(dev.DeviceID).
		NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).
		Snapshot(false).Do(ctx, service.Sleep(0))
}

// InputText inputs text quickly.
func (dev *Device) InputText(ctx context.Context, text string) {
	inputTextCmd := fmt.Sprintf("shell input text \"%s\"", text)
	dev.Client.ExecCommand(dev.DeviceID, inputTextCmd).Do(ctx)
}
