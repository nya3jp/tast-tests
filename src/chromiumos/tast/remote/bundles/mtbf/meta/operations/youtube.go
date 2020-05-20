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
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

// OpenYoutubeAndPlay open Youtube and play
func OpenYoutubeAndPlay(ctx context.Context, s *testing.State, dut *utils.Device) error {
	// Play fixed video url from config
	youtubeURL := common.GetVar(ctx, s, "cats.youtubeURL")
	youtubeTitle := common.GetVar(ctx, s, "cats.youtubeTitle")
	coordinate := common.GetVar(ctx, s, "coordinate.youtube")

	s.Log("Open Youtube app")
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.google.android.apps.youtube.app.WatchWhileActivity",
		"com.google.android.youtube").Do(ctx); err != nil {
		return err
	}
	testing.Sleep(ctx, 2*time.Second)

	dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx, service.Suppress())
	s.Log("If the app is started for the first time")
	// If the app is started for the first time, do following operation to init.
	dut.Client.UIAClick(dut.DeviceID).Selector("text=TRY IT FREE").Do(ctx, service.Suppress())
	dut.Client.UIAClick(dut.DeviceID).Selector("textstartwith=SKIP").Do(ctx, service.Suppress())
	dut.Client.UIAClick(dut.DeviceID).Selector("descstartwith=SKIP").Do(ctx, service.Suppress())
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=Close").Do(ctx, service.Suppress())

	// Wait for the app loading.

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
	dut.Client.InputText(dut.DeviceID, youtubeURL).Do(ctx)
	s.Log("Press Enter")
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventENTER).Do(ctx)

	if isEnetrList, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "descstartwith="+youtubeTitle, 12000, ui.ObjEventTypeAppear).Do(ctx); isEnetrList {
		//play the video from search result list
		dut.Client.UIAClick(dut.DeviceID).Selector("descstartwith=" + youtubeTitle).Do(ctx)
	} else {
		return mtbferrors.New(mtbferrors.YoutubeTitle, nil)
	}

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
	coordinate := common.GetVar(ctx, s, "coordinate.youtube")
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
