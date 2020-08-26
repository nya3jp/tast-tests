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

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF004NewsVideoPlaying,
		Desc:         "ARC++ Test News video apps",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"meta.coordNews", "meta.requestURL"},
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps: []string{
			"tast.mtbf.multimedia.VolumeService",
			"tast.mtbf.svc.CommService",
		},
	})
}

/*
Precondition:
Install google news app from the Play Store.

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

func MTBF004NewsVideoPlaying(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF004NewsVideoPlaying",
		Description: "ARC++ Test News video apps",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)
		if mtbferr := verifyNewsVideoPlaying(ctx, s, dutDev); mtbferr != nil {
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
		client.Comments("Recover env").Do(ctx)
		client.Press(dutID, ui.OprKeyEventCANCEL).Times(5).Do(ctx)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

// verifyNewsVideoPlaying verify facebook video is playing
func verifyNewsVideoPlaying(ctx context.Context, s *testing.State, dut *utils.Device) error {
	const (
		act = "com.google.apps.dots.android.app.activity.CurrentsStartActivity"
		pkg = "com.google.android.apps.magazines"

		packageSel   = "packagename=" + pkg
		allowPermSel = "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button"
		viewTextSel  = "text=View all and manage"

		newsIDPrefix    = "ID=com.google.android.apps.magazines:id/"
		followingTabSel = newsIDPrefix + "tab_following"
		embedVideoSel   = newsIDPrefix + "embed_video_view"
		firstCardSel    = newsIDPrefix + "card::index=0"
		progressBarSel  = newsIDPrefix + "progress_bar"
		pauseBtnSel     = newsIDPrefix + "embed_video_play_button::desc=Pause Video"
		retrySel        = newsIDPrefix + "video_error_retry::text=RETRY"
	)

	dut.Client.StartMainActivity(dut.DeviceID, act, pkg).Do(ctx)
	dut.Client.Delay(2000).Do(ctx)
	dut.AllowPermission(ctx, allowPermSel)

	s.Log("Verify whether News App is launched")
	isEnterApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, packageSel, 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isEnterApp {
		return mtbferrors.New(mtbferrors.GoogleNewsApp, nil)
	}

	s.Log("Click 'Following'")
	dut.Client.UIAClick(dut.DeviceID).Selector(followingTabSel).Do(ctx)

	s.Log("Scroll to the bottom")
	dut.Client.SwipePage(dut.DeviceID, ui.ScrollDirectionsDOWN).Times(3).Do(ctx)

	s.Log("Click 'View all and manage'")
	isSaveVideo, _ := dut.Client.UIAObjEventWait(dut.DeviceID, viewTextSel, 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isSaveVideo {
		return mtbferrors.New(mtbferrors.SetUpGoogleNewsEnv, nil)
	}
	dut.Client.UIAClick(dut.DeviceID).Selector(viewTextSel).Do(ctx)

	s.Log("Click the first video")
	video, _ := dut.Client.UIASearchElement(dut.DeviceID, embedVideoSel).ParentSelector(firstCardSel).Do(ctx)
	if !video.Found {
		s.Log("Retry searching the video after 5 seconds")
		testing.Sleep(ctx, time.Second*5)
		video, _ = dut.Client.UIASearchElement(dut.DeviceID, embedVideoSel).ParentSelector(firstCardSel).Do(ctx)
		if !video.Found {
			return mtbferrors.New(mtbferrors.GoogleNewsVideoNotPlay, nil)
		}
	}
	coord := fmt.Sprintf("%d,%d", video.CenterX, video.CenterY)
	dut.ClickCoordinate(ctx, coord)

	s.Log("Wait for the video refresh")
	dut.Client.UIAObjEventWait(dut.DeviceID, progressBarSel, 60000, ui.ObjEventTypeDisappear).Do(ctx)

	// 2.1 Video can be played.
	// Trigger the page with control video button.
	s.Log("Verify whether the video is playing")
	dut.ClickCoordinate(ctx, coord)
	isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, pauseBtnSel, 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !isPlay {
		dut.Client.Delay(5000).Do(ctx)
		s.Log("Maybe not playing, verify again")
		dut.ClickCoordinate(ctx, coord)
		isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, pauseBtnSel, 5000, ui.ObjEventTypeAppear).Do(ctx)
		if !isPlay {
			isCrash, _ := dut.Client.UIAObjEventWait(dut.DeviceID, retrySel, 3000, ui.ObjEventTypeAppear).Do(ctx)
			if isCrash {
				return mtbferrors.New(mtbferrors.GoogleNewsCrash, nil)
			}
			return mtbferrors.New(mtbferrors.GoogleNewsVideoNotPlay, nil)
		}
	}

	return nil
}
