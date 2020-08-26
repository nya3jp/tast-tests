// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package operations

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/testing"
)

// OpenYoutubeAndPlay open Youtube and play
func OpenYoutubeAndPlay(ctx context.Context, s *testing.State, dut *utils.Device) error {
	// Play fixed video url from config
	youtubeURL := s.RequiredVar("meta.youtubeURL")
	youtubeTitle := s.RequiredVar("meta.youtubeTitle")
	coordinate := s.RequiredVar("meta.coordYoutube")

	s.Log("Open Youtube app")
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.google.android.apps.youtube.app.WatchWhileActivity",
		"com.google.android.youtube").Do(ctx); err != nil {
		return err
	}
	testing.Sleep(ctx, 2*time.Second)

	dut.ClickSelector(ctx, "text=Close app")

	s.Log("If the app is started for the first time, do following operation to init")
	dut.ClickSelector(ctx, "text=TRY IT FREE")
	dut.ClickSelector(ctx, "textstartwith=SKIP")
	dut.ClickSelector(ctx, "descstartwith=SKIP")
	dut.ClickSelector(ctx, "desc=Close")

	s.Log("Wait for the app loading")
	dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/load_progress", 60000, ui.ObjEventTypeDisappear)

	// Verify whether enter to app.
	if isEnetrApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/youtube_logo", 60000, ui.ObjEventTypeAppear).Do(ctx); !isEnetrApp {
		return mtbferrors.New(mtbferrors.YoutubeApp, nil)
	}

	// Click search button
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.youtube:id/menu_item_1").Do(ctx)

	// Enter search textfield
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.youtube:id/search_edit_text").Do(ctx)

	// Keypress fixed url and send enter key
	s.Log("Input the URL and press Enter")
	dut.InputText(ctx, youtubeURL)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventENTER).Do(ctx)

	if found, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "descstartwith="+youtubeTitle, 12000, ui.ObjEventTypeAppear).Do(ctx); !found {
		return mtbferrors.New(mtbferrors.YoutubeTitle, nil)
	}

	//play the video from search result list
	dut.Client.UIAClick(dut.DeviceID).Selector("descstartwith=" + youtubeTitle).Do(ctx)

	// Wait for the video loading.
	s.Log("Wait for the video loading")
	dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/player_loading_view_thin", 60000, ui.ObjEventTypeDisappear)
	// Skip ad
	// verify_and_skip_ad(device_id)
	// Trigger the page with control video button.
	dut.Client.UIAClick(dut.DeviceID).Coordinate(coordinate).Snapshot(false).Do(ctx, service.Sleep(0))
	dut.Client.ScrollListItem(dut.DeviceID, ui.ScrollDirectionsLEFT, ui.NewUiNodePropDesc().Class("android.widget.SeekBar")).Do(ctx)

	testing.Sleep(ctx, 2*time.Second)
	// Trigger the page with control video button.
	dut.Client.UIAClick(dut.DeviceID).Coordinate(coordinate).Snapshot(false).Do(ctx, service.Sleep(0))

	// Verify the video is playing
	s.Log("Verify the video is playing")
	if isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/player_control_play_pause_replay_button::desc=Pause video", 5000, ui.ObjEventTypeAppear).Do(ctx); !isPlay {
		return mtbferrors.New(mtbferrors.CannotPlayYoutubeVideo, nil)
	}
	return nil
}

// VerifyYoutubePlaying verify youtube is playing
func VerifyYoutubePlaying(ctx context.Context, s *testing.State, dut *utils.Device) error {
	coordinate := s.RequiredVar("meta.coordYoutube")
	dut.Client.UIAClick(dut.DeviceID).Coordinate(coordinate).Snapshot(false).Do(ctx)
	if isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/player_control_play_pause_replay_button::desc=Pause video", 5000, ui.ObjEventTypeAppear).Do(ctx); !isPlay {
		return mtbferrors.New(mtbferrors.YoutubeNotPlay, nil)
	}
	return nil
}

// CloseYoutube close app
func CloseYoutube(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	// Back to main page of Youtube.
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(1).Do(ctx)

	// Close the opened video.
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.youtube:id/floaty_close_button").Do(ctx)

	// Exit youtube.
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(5).Do(ctx)
}
