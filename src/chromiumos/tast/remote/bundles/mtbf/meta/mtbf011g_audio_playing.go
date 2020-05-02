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
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF011GAudioPlaying,
		Desc:     "ARC++ Test top android audio apps",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "googlemusic",
			Val:  "mtbf011playgooglemusic",
		}, {
			Name: "spotify",
			Val:  "mtbf011playspotify",
		}, {
			Name: "youtubemusic",
			Val:  "mtbf011playyoutubemusic",
		}},
		Vars: []string{"cats.requestURL"},
	})
}

func drive011GPlayGoogleMusic(ctx context.Context, dut *utils.Device) error {
	dut.OpenGoogleMusicAndPlay(ctx)
	return nil
}

func cleanup011GGoogleMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.music:id/pause").Do(ctx, service.Sleep(time.Second*2))
	}

	dut.PressCancelButton(ctx, 1)
}

func drive011GPlaySpotify(ctx context.Context, dut *utils.Device) error {
	dut.OpenSpotifyAndPlay(ctx)
	return nil
}

func cleanup011GSpotify(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.spotify.music:id/play_pause_button::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.spotify.music:id/play_pause_button::desc=Pause", 60000, ui.ObjEventTypeAppear).Do(ctx)
		if play {
			//dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.spotify.music:id/play_pause_button").Do(ctx, service.Suppress())
			utils.ClickSelector(ctx, dut, "ID=com.spotify.music:id/play_pause_button")
		}
	}

	dut.PressCancelButton(ctx, 4)
}

func drive011GPlayYoutubeMusic(ctx context.Context, dut *utils.Device) error {
	dut.OpenYoutubeMusicAndPlay(ctx)
	return nil
}

func cleanup011GYoutubeMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.apps.youtube.music:id/play_pause_replay_button::desc=Play video", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/play_pause_replay_button").Do(ctx)
	}

	dut.PressCancelButton(ctx, 4)
}

func drive011GDUTVolumeTuning(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	testNames := []string{
		"video.MTBF004AdjustVolume.100",
		"video.MTBF004AdjustVolume.10",
	}

	for _, testName := range testNames {
		if mtbferr := tastrun.RunTestWithFlags(ctx, s, flags, testName); mtbferr != nil {
			return mtbferr
		}
	}

	return nil
}

func MTBF011GAudioPlaying(ctx context.Context, s *testing.State) {
	subcase := s.Param().(string)

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

	runCaseMap := map[string]func(ctx context.Context, dut *utils.Device) error{
		"mtbf011playgooglemusic":  drive011GPlayGoogleMusic,
		"mtbf011playspotify":      drive011GPlaySpotify,
		"mtbf011playyoutubemusic": drive011GPlayYoutubeMusic,
	}

	cleanCaseMap := map[string]func(ctx context.Context, dut *utils.Device){
		"mtbf011playgooglemusic":  cleanup011GGoogleMusic,
		"mtbf011playspotify":      cleanup011GSpotify,
		"mtbf011playyoutubemusic": cleanup011GYoutubeMusic,
	}

	report, _, err := androidTest.RunDelegate(ctx, sdk.CaseDescription{
		Name:        "case_name",
		Description: "A new case",
		ReportPath:  "report/path",
		DutID:       dutID,
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		if err := runCaseMap[subcase](ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive011GDUTVolumeTuning(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanCaseMap[subcase](ctx, dutDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
