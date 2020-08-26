// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF031PlayAudioFromAndroidAndChromeOS,
		Desc:         "ARC++ Playing audio from Android and ChromeOS simultaneously",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"format_m4a.m4a", "format_mp3.mp3"},
		Vars:         []string{"meta.requestURL"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_p"},
		ServiceDeps: []string{
			"tast.mtbf.multimedia.AudioPlayer",
			"tast.mtbf.svc.CommService",
		},
	})
}

func drive031DUTMediaPlayer(ctx context.Context, player multimedia.AudioPlayerClient, s *testing.State) error {
	s.Log("Prepare to play m4a file")
	req := multimedia.FileRequest{
		Filepath: "audios/format_m4a.m4a",
	}
	if _, err := player.OpenInDownloads(ctx, &req); err != nil {
		return err
	}

	testing.Sleep(ctx, 2*time.Second)
	s.Log("Verify the audio is playing by built-in player")

	_, err := player.MustPlaying(ctx, &multimedia.TimeoutRequest{Seconds: 3})
	return err
}

func drive031DUTCheckMediaPlayerStatus(ctx context.Context, player multimedia.AudioPlayerClient, s *testing.State) error {
	if _, err := player.Focus(ctx, &empty.Empty{}); err != nil {
		return err
	}
	testing.Sleep(ctx, 2*time.Second) // for possible UI update delay

	s.Log("Verify audio player is paused")
	if _, err := player.MustPaused(ctx, &multimedia.TimeoutRequest{Seconds: 5}); err != nil {
		return err
	}

	s.Log("Resume what's audio player is playing")
	if _, err := player.Play(ctx, &empty.Empty{}); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.AudioPlaying, err))
	}
	testing.Sleep(ctx, time.Second)

	s.Log("Verify audio player is playing")
	_, err := player.MustPlaying(ctx, &multimedia.TimeoutRequest{Seconds: 5})
	return err
}

func drive031DUTVLCPlayer(ctx context.Context, dut *utils.Device) error {
	dut.OpenVLCAndEnterToDownload(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=audios").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::textcontains=mp3").Do(ctx)
	//dut.Client.UIAClick(dut.DeviceID).Selector("textstartwith=Got it").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "textstartwith=Got it")

	play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !play {
		return mtbferrors.New(mtbferrors.VLCAppNotPlay, nil)
	}

	return nil
}

func drive031DUTVerifyVLCPlayer(ctx context.Context, dut *utils.Device) error {
	dut.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		return mtbferrors.New(mtbferrors.VLCAppNotPause, nil)
	}

	return nil
}

func drive031DUTCleanup(ctx context.Context, dut *utils.Device) error {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/header_play_pause").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
	dut.ClickSelector(ctx, "text=OK")
	dut.PressCancelButton(ctx, 1)
	dut.ClickSelector(ctx, "text=OK")
	return nil
}

func MTBF031PlayAudioFromAndroidAndChromeOS(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF031PlayAudioFromAndroidAndChromeOS",
		Description: "ARC++ Playing audio from Android and ChromeOS simultaneously",
		Timeout:     5 * time.Minute,
	}

	common.AudioFilesPrepare(ctx, s, []string{"format_m4a.m4a", "format_mp3.mp3"})

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer c.Close(ctx)

		player := multimedia.NewAudioPlayerClient(c.Conn)
		defer player.CloseAll(ctx, &empty.Empty{})

		testing.ContextLog(ctx, "Open built-in media player and play")
		if mtbferr := drive031DUTMediaPlayer(ctx, player, s); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Open VLC player and play")
		if mtbferr := drive031DUTVLCPlayer(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Check media player status")
		if mtbferr := drive031DUTCheckMediaPlayerStatus(ctx, player, s); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Verify VLC player still play")
		if mtbferr := drive031DUTVerifyVLCPlayer(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		drive031DUTCleanup(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
