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
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/operations"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF004YoutubeVideoPlaying,
		Desc:         "ARC++ Test YouTube video apps",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"meta.coordYoutube", "meta.youtubeURL", "meta.youtubeTitle", "meta.requestURL"},
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps: []string{
			"tast.mtbf.multimedia.VolumeService",
			"tast.mtbf.svc.CommService",
		},
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
			common.Fatal(ctx, s, mtbferr)
		}

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		vsc := multimedia.NewVolumeServiceClient(cl.Conn)
		if _, mtbferr := vsc.Set(ctx, &multimedia.VolumeRequest{Value: 10}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		if _, mtbferr := vsc.Set(ctx, &multimedia.VolumeRequest{Value: 100}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
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
	const (
		idPrefix      = "ID=com.google.android.youtube:id/"
		timeBarSel    = idPrefix + "time_bar_current_time"
		pauseBtnSel   = idPrefix + "player_control_play_pause_replay_button::desc=Pause video"
		playerViewSel = idPrefix + "player_view::index=0"
		containerSel  = idPrefix + "player_fragment_container"
		listViewSel   = idPrefix + "bottom_sheet_list_view"
		listItemSel   = idPrefix + "list_item_text"
		fullscreenSel = idPrefix + "fullscreen_button::desc=Enter fullscreen"
	)

	if mtbferr := operations.OpenYoutubeAndPlay(ctx, s, dut); mtbferr != nil {
		s.Fatal(mtbferr)
	}

	view, _ := dut.Client.UIASearchElement(dut.DeviceID, playerViewSel).ParentSelector(containerSel).Do(ctx)
	coord := fmt.Sprintf("%d,%d", view.BoundRight-100, view.CenterY)
	s.Logf("coord = %s", coord)
	// Get start time before seek
	dut.ClickCoordinate(ctx, coord)
	startTime, _ := dut.Client.GetWidgetText2(dut.DeviceID, timeBarSel).Do(ctx)
	s.Logf("startTime = %s", startTime)

	// 3. Seek video to different positions.
	// Four consecutive clicks will fast-forward video for 30 seconds
	dut.Client.UIAClick(dut.DeviceID).
		Coordinate(coord).Times(4).Intervals(50).
		Do(ctx, service.Sleep(time.Second*2))

	// Verify whether the video continues to play.
	s.Log("Verify playing")
	dut.ClickCoordinate(ctx, coord)
	isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, pauseBtnSel, 5000, ui.ObjEventTypeAppear).Do(ctx, service.Sleep(time.Second*5))
	if !isPlay {
		s.Log("Verify playing again")
		dut.ClickCoordinate(ctx, coord)
		isPlay, _ = dut.Client.UIAObjEventWait(dut.DeviceID, pauseBtnSel, 5000, ui.ObjEventTypeAppear).Do(ctx, service.Sleep(time.Second*5))
		if !isPlay {
			return mtbferrors.New(mtbferrors.CannotPlayYoutubeVideo, nil)
		}
	}

	// 3.1 Video should play from the seek position.
	s.Log("Verify seek position")
	// Get end time after seek
	dut.ClickCoordinate(ctx, coord)
	seekTime, _ := dut.Client.GetWidgetText2(dut.DeviceID, timeBarSel).Do(ctx)
	s.Logf("seekTime = %s", seekTime)

	// Verify the diff between end and start should be at least 25s.
	diff := getSeconds(seekTime) - getSeconds(startTime)
	if diff < 25 {
		return mtbferrors.New(mtbferrors.VerifyYoutubeSeek, nil)
	}
	comment := fmt.Sprintf("Fast-forward video for %d seconds.", diff)
	dut.Client.Comments(comment).Do(ctx, service.Sleep(time.Second*5))

	// 4. Change resolution settings if supported
	s.Log("Verify resolution")
	dut.ClickCoordinate(ctx, coord)
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=More options").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=Quality").Do(ctx)
	// Find a supported resolution for the video
	resolution, _ := dut.Client.UIASearchSequentialElement(dut.DeviceID, listViewSel, 2).TargetSelector(listItemSel).Do(ctx)
	if !resolution.Found {
		return mtbferrors.New(mtbferrors.VerifyYoutubeResolution, nil)
	}
	targetResolutionSel := "text=" + resolution.Text
	dut.ClickSelector(ctx, targetResolutionSel)
	dut.ClickCoordinate(ctx, coord)
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=More options").Do(ctx)
	isResolutionChanged, _ := dut.Client.UIAObjEventWait(dut.DeviceID, idPrefix+"list_item_text_secondary::"+targetResolutionSel, 2000, ui.ObjEventTypeAppear).Do(ctx)
	// 4.1 Video should play with new resolution.
	if !isResolutionChanged {
		return mtbferrors.New(mtbferrors.VerifyYoutubeResolution, nil)
	}
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Do(ctx)

	// 5. Play in full screen if supported
	// Get the UI selector center coordinate before full screen
	viewBeforeFull := view
	s.Logf("viewBeforeFull = %+v", viewBeforeFull)

	// Click full screen button
	dut.ClickCoordinate(ctx, coord)
	s.Log("Click fullscreen button")
	dut.Client.UIAClick(dut.DeviceID).Selector(fullscreenSel).Do(ctx)

	// 5.1 Video should play with full screen.
	// Get the UI selector center coordinate after full screen
	viewAfterFull, _ := dut.Client.UIASearchElement(dut.DeviceID, playerViewSel).ParentSelector(containerSel).Do(ctx)
	s.Logf("viewAfterFull = %+v", viewAfterFull)

	// Verify center coordinate after full screen is bigger than before
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
