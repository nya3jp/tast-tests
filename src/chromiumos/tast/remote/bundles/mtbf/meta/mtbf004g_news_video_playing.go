// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF004GNewsVideoPlaying,
		Desc:     "ARC++ Test News video apps",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"coordinate.news", "cats.requestURL"},
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

func MTBF004GNewsVideoPlaying(ctx context.Context, s *testing.State) {
	dutID, err := s.DUT().GetARCDeviceID(ctx)
	if err != nil {
		s.Fatal(mtbferrors.OSNoArcDeviceID, err)
	}

	addr, err := common.CatsNodeAddress(ctx, s)
	if err != nil {
		s.Fatal("Failed to get cats node addr: ", err)
	}

	androidTest, err := sdk.New(addr)
	if err != nil {
		s.Fatal("Failed to new androi test: ", err)
	}

	if err := common.CatsMTBFLogin(ctx, s); err != nil {
		s.Fatal("Failed to do MTBFLogin: ", err)
	}

	report, _, mtbferr := androidTest.RunDelegate(ctx, sdk.CaseDescription{
		Name:        "case_name",
		Description: "A new case",
		ReportPath:  "report/path",
		DutID:       dutID,
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)
		if err := verifyNewsVideoPlaying(ctx, s, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := common.DriveDUT(ctx, s, "video.MTBF004AdjustVolume.10"); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := common.DriveDUT(ctx, s, "video.MTBF004AdjustVolume.100"); err != nil {
			utils.FailCase(ctx, client, err)
		}
		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		closeNews(ctx, dutDev)
		return nil, nil
	})

	_ = report

	s.Log(report)
	if mtbferr != nil {
		s.Error(mtbferr)
	}
}

// verifyNewsVideoPlaying verify facebook video is playing
func verifyNewsVideoPlaying(ctx context.Context, s *testing.State, dut *utils.Device) error {
	coordinate := common.GetVar(ctx, s, "coordinate.news")
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.google.apps.dots.android.app.activity.CurrentsStartActivity",
		"com.google.android.apps.magazines").Do(ctx); err != nil {
		return err
	}

	testing.Sleep(ctx, 2*time.Second)
	dut.AllowPermission(ctx, "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button")
	s.Log("Enter App")
	isEnterApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "packagename=com.google.android.apps.magazines", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isEnterApp {
		return mtbferrors.New(mtbferrors.GoogleNewsApp, nil)
	}

	// Click "Following"
	s.Log("Click 'Following'")
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.apps.magazines:id/tab_following::index=2").Do(ctx)

	// Click one video in "Saved stories"
	s.Log("Click 'Saved stories'")
	dut.Client.SwipePage(dut.DeviceID, ui.ScrollDirectionsDOWN).Times(3).Do(ctx)
	isSaveVideo, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=View all and manage", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isSaveVideo {
		return mtbferrors.New(mtbferrors.SetUpGoogleNewsEnv, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("text=View all and manage").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Coordinate(coordinate).Snapshot(false).Do(ctx, service.Sleep(0))

	// Wait for the video refresh
	s.Log("Wait for the video refresh")
	dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.apps.magazines:id/progress_bar", 60000, ui.ObjEventTypeDisappear)

	// 2.1 Video can be played.
	// Trigger the page with control video button.
	dut.Client.UIAClick(dut.DeviceID).Coordinate(coordinate).Snapshot(false).Do(ctx, service.Sleep(0))

	// Verify the video is playing.
	s.Log("Verify the video is playing")
	isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.apps.magazines:id/embed_video_play_button::desc=Pause Video", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !isPlay {
		isCrash, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.apps.magazines:id/video_error_retry::text=RETRY", 3000, ui.ObjEventTypeAppear).Do(ctx)
		if isCrash {
			return mtbferrors.New(mtbferrors.GoogleNewsCrash, nil)
		}
		return mtbferrors.New(mtbferrors.GoogleNewsVideoNotPlay, nil)
	}
	return nil
}

// closeNews close app
func closeNews(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(5).Do(ctx)
}
