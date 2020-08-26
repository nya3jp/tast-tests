// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF030SwitchMusicPlayer,
		Desc:     "Android apps should gain focus (ARC++)",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.requestURL", "video.youtubeVideo"},
		Data:     []string{"format_m4a.m4a"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
			"tast.mtbf.svc.WebService",
			"tast.mtbf.multimedia.YoutubeService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

// drive030PlayGoogleMusicAgain plays google music again after open chrome browser and navigator to youtube.
func drive030PlayGoogleMusicAgain(ctx context.Context, dut *utils.Device) error {
	// # 3.Go to Google Play Music and click the play button
	testing.ContextLog(ctx, "Play google music again")
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.android.music.activitymanagement.TopLevelActivity",
		"com.google.android.music").Do(ctx); err != nil {
		return err
	}
	isPause, _ := dut.Client.UIAObjEventWait(dut.DeviceID,
		"id=com.google.android.music:id/pause::desc=Play", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if isPause {
		// 3.Go to Google Play Music and click the play button to resume playing and Observe behavior
		dut.Client.UIAClick(dut.DeviceID).Selector("id=com.google.android.music:id/pause").Do(ctx)
		isStartRadio, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=START RADIO", 2000, ui.ObjEventTypeAppear).Do(ctx)
		if isStartRadio {
			dut.Client.UIAClick(dut.DeviceID).Selector("text=START RADIO").Do(ctx)
		}
	} else {
		return mtbferrors.New(mtbferrors.VLCAppNotPause, nil)
	}
	return nil
}

// cleanup030GoogleMusic cleans up google music app.
func cleanup030GoogleMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.EnterToAppAndVerify(ctx, "com.android.music.activitymanagement.TopLevelActivity", "com.google.android.music", "packagename=com.google.android.music")
	ok, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !ok {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.music:id/pause").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
}

func MTBF030SwitchMusicPlayer(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF030SwitchMusicPlayer",
		Description: "Android apps should gain focus (ARC++)",
		Timeout:     time.Minute * 5,
	}

	common.AudioFilesPrepare(ctx, s, []string{"format_m4a.m4a"})

	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer cl.Close(ctx)

	videoURL := common.Add1SecondForURL(s.RequiredVar("video.youtubeVideo"))
	youtubeClient := multimedia.NewYoutubeServiceClient(cl.Conn)
	webClient := svc.NewWebServiceClient(cl.Conn)

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := dutDev.OpenGoogleMusicAndPlay(ctx); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		req := &multimedia.PlayYoutubeVideoRequest{URL: videoURL}
		if _, mtbferr := youtubeClient.PlayYoutubeVideo(ctx, req); err != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		s.Log("Play for 5 more seconds")
		testing.Sleep(ctx, time.Second*5)

		if mtbferr := drive030PlayGoogleMusicAgain(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cleanup030GoogleMusic(ctx, dutDev)

		webClient.CloseURL(ctx, &svc.CloseURLRequest{Url: videoURL})

		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
