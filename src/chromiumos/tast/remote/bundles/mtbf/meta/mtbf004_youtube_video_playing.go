// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/operations"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF004YoutubeVideoPlaying,
		Desc:     "ARC++ Test YouTube video apps",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"coordinate.youtube", "cats.youtubeURL", "cats.youtubeTitle", "cats.requestURL"},
	})
}

/*
Precondition:
Install youtube, vimeo, vevo, facebook, news apps from the Play Store.

Procedure:
1. Open above mentioned apps one after other and run below steps.
2. Check video playback.
3. Seek video to different positions.
4. Change resolution settings if supported
5. Play in full screen if supported
6. Observe audio controls behavior.

Verification:
2.1 Video can be played.
3.1 Video should play from the seek position.
4.1 Video should play with new resolution.
5.1 Video should play with full screen.
6.1 Audio levels should be effected only with ChromeOS audio controls. (ie. Device volume level doesn't change if changing volues inside Android APP)
*/

func MTBF004YoutubeVideoPlaying(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF004YoutubeVideoPlaying",
		Description: "ARC++ Test YouTube video apps",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := verifyYoutubeVideoPlaying(ctx, s, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := common.DriveDUT(ctx, s, "video.MTBF004AdjustVolume.10"); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := common.DriveDUT(ctx, s, "video.MTBF004AdjustVolume.100"); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		operations.CloseYoutube(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

func getSeconds(time string) int {
	var mm int
	var ss int
	fmt.Sscanf(time, "%d:%d", &mm, &ss)
	return mm*60 + ss
}

// verifyYoutubeVideoPlaying verify youtube video is playing
func verifyYoutubeVideoPlaying(ctx context.Context, s *testing.State, dut *utils.Device) error {
	coordinate := common.GetVar(ctx, s, "coordinate.youtube")
	operations.OpenYoutubeAndPlay(ctx, s, dut)
	testing.Sleep(ctx, 2*time.Second)

	// Get start time before seek
	startTime := dut.GetCurrentPlayingTimeOfVideo(ctx, "302,257", "com.google.android.youtube:id/time_bar_current_time")

	// 3. Seek video to different positions.
	// Four consecutive clicks will seek video for 30 seconds
	dut.Client.UIAClick(dut.DeviceID).Coordinate(coordinate).Times(4).Intervals(20).Do(ctx)
	testing.Sleep(ctx, 2*time.Second)

	// Trigger the page with control video button.

	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coordinate)).Do(ctx, service.Sleep(0))

	// Verify whether the video continues to play.
	s.Log("Verify playing")
	isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/player_control_play_pause_replay_button::desc=Pause video", 15000, ui.ObjEventTypeAppear).Do(ctx)
	if !isPlay {
		// retry check playing

		dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coordinate)).Do(ctx, service.Sleep(0))
		isPlay, _ = dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/player_control_play_pause_replay_button::desc=Pause video", 15000, ui.ObjEventTypeAppear).Do(ctx)
		if !isPlay {
			return mtbferrors.New(mtbferrors.CannotPlayYoutubeVideo, nil)
		}
	}
	testing.Sleep(ctx, 5*time.Second)

	// 3.1 Video should play from the seek position.
	s.Log("Verify seek position")
	// Get end time after seek
	seekTime := dut.GetCurrentPlayingTimeOfVideo(ctx, "302,257", "com.google.android.youtube:id/time_bar_current_time")
	// Verify the diff between end and start should bigger than 25s at least.
	diff := getSeconds(seekTime) - getSeconds(startTime) //seekTime - startTime //int((seekTime - startTime).total_seconds())
	if diff < 15 {
		return mtbferrors.New(mtbferrors.VerifyYoutubeSeek, nil)
	}
	comment := fmt.Sprintf("Seek video for %d seconds.", diff)
	dut.Client.Comments(comment).Do(ctx)

	testing.Sleep(ctx, 5*time.Second)
	// 4. Change resolution settings if supported
	s.Log("Verify resolution")
	//  Trigger the page with control video button.

	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coordinate)).Do(ctx, service.Sleep(0))
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=More options").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=Quality").Do(ctx)
	// Find a supported resolution for the video
	resolution, _ := dut.Client.UIASearchSequentialElement(dut.DeviceID,
		"ID=com.google.android.youtube:id/bottom_sheet_list_view", 2).TargetSelector(
		"ID=com.google.android.youtube:id/list_item_text").Do(ctx)

	if resolution.Found {
		targetChangeResolution := resolution.Text
		selector := fmt.Sprintf("%s%s", "text=", targetChangeResolution)
		s.Log(selector)
		// Change resolution
		dut.Client.UIAClick(dut.DeviceID).Selector(selector).Do(ctx)
		dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coordinate)).Do(ctx, service.Sleep(0))
		dut.Client.UIAClick(dut.DeviceID).Selector("desc=More options").Do(ctx)
		// Trigger the page with control video button.
		isResolutionChanged, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.youtube:id/list_item_text_secondary::"+selector, 2000, ui.ObjEventTypeAppear).Do(ctx)
		// 4.1 Video should play with new resolution.
		if !isResolutionChanged {
			return mtbferrors.New(mtbferrors.VerifyYoutubeResolution, nil)
		}
	}

	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(1).Do(ctx)

	// 5. Play in full screen if supported
	// Get the UI selector center coordinate=coordinate before full screen
	viewBeforeFull, _ := dut.Client.UIASearchElement(dut.DeviceID,
		"ID=com.google.android.youtube:id/player_view::index=0").ParentSelector(
		"ID=com.google.android.youtube:id/player_fragment_container").Do(ctx)
	s.Log(viewBeforeFull)
	// Trigger the page with control video button.
	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coordinate)).Do(ctx, service.Sleep(0))

	// Click full screen button
	s.Log("Click fullscreen")
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.youtube:id/fullscreen_button::desc=Enter fullscreen").Do(ctx)

	//5.1 Video should play with full screen.
	// Get the UI selector center coordinate=coordinate after full screen
	viewAfterFull, _ := dut.Client.UIASearchElement(dut.DeviceID,
		"ID=com.google.android.youtube:id/player_view::index=0").ParentSelector(
		"ID=com.google.android.youtube:id/player_fragment_container").Do(ctx)
	// Verify center coordinate=coordinate after full screen is bigger than before
	beforeX := viewBeforeFull.CenterX
	afterX := viewAfterFull.CenterX
	beforeY := viewBeforeFull.CenterY
	afterY := viewAfterFull.CenterY

	if viewBeforeFull.Found && viewAfterFull.Found {
		if beforeX < afterX && beforeY < afterY {
			dut.Client.Comments("Screen becomes full screen.").Do(ctx)
		} else {
			dut.Client.Comments("Can't verify the resolution change.").Do(ctx)
		}
	}

	return nil
}
