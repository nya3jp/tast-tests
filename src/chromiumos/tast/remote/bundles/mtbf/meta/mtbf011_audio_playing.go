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
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
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
		Vars:         []string{"meta.requestURL"},
		Data:         []string{"format_m4a.m4a"},
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps: []string{
			"tast.mtbf.multimedia.VolumeService",
			"tast.mtbf.svc.CommService",
		},
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

	dut.PressCancelButton(ctx, 6)
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

	common.AudioFilesPrepare(ctx, s, []string{"format_m4a.m4a"})

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := runCaseMap[subcase](ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		vsc := multimedia.NewVolumeServiceClient(cl.Conn)
		if _, mtbferr := vsc.Set(ctx, &multimedia.VolumeRequest{Value: 100}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		if _, mtbferr := vsc.Set(ctx, &multimedia.VolumeRequest{Value: 10}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
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
