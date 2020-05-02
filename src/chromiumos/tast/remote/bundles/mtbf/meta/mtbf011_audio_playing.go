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

	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF011AudioPlaying,
		Desc:     "ARC++ Test top android audio apps",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name: "googlemusic",
			Val:  "googlemusic",
		}, {
			Name: "spotify",
			Val:  "spotify",
		}, {
			Name: "youtubemusic",
			Val:  "youtubemusic",
		}},
		Vars: []string{"cats.requestURL"},
	})
}

func drive011PlayGoogleMusic(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Open Google music and play")
	dut.OpenGoogleMusicAndPlay(ctx)
	return nil
}

func cleanup011GoogleMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.music:id/pause").Do(ctx, service.Sleep(time.Second*2))
	}

	dut.PressCancelButton(ctx, 1)
}

func drive011PlaySpotify(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Open Spotify and play")
	dut.OpenSpotifyAndPlay(ctx)
	return nil
}

func cleanup011Spotify(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.spotify.music:id/play_pause_button::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.spotify.music:id/play_pause_button::desc=Pause", 60000, ui.ObjEventTypeAppear).Do(ctx)
		if play {
			//dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.spotify.music:id/play_pause_button").Do(ctx, service.Suppress())
			dut.ClickSelector(ctx, "ID=com.spotify.music:id/play_pause_button")
		}
	}

	dut.PressCancelButton(ctx, 4)
}

func drive011PlayYoutubeMusic(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Open Youtube music and play")
	dut.OpenYoutubeMusicAndPlay(ctx)
	return nil
}

func cleanup011YoutubeMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.apps.youtube.music:id/play_pause_replay_button::desc=Play video", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.apps.youtube.music:id/play_pause_replay_button").Do(ctx)
	}

	dut.PressCancelButton(ctx, 4)
}

func drive011DUTVolumeTuning(ctx context.Context, s *testing.State) error {
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

func MTBF011AudioPlaying(ctx context.Context, s *testing.State) {
	subcase := s.Param().(string)

	runCaseMap := map[string]func(ctx context.Context, dut *utils.Device) error{
		"googlemusic":  drive011PlayGoogleMusic,
		"spotify":      drive011PlaySpotify,
		"youtubemusic": drive011PlayYoutubeMusic,
	}

	cleanCaseMap := map[string]func(ctx context.Context, dut *utils.Device){
		"googlemusic":  cleanup011GoogleMusic,
		"spotify":      cleanup011Spotify,
		"youtubemusic": cleanup011YoutubeMusic,
	}

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF011AudioPlaying." + subcase,
		Description: "ARC++ Test top android audio apps",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := runCaseMap[subcase](ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := drive011DUTVolumeTuning(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanCaseMap[subcase](ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
