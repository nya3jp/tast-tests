// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF031PlayAudioFromAndroidAndChromeOS,
		Desc:     "ARC++ Playing audio from Android and ChromeOS simultaneously",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive031DUTMediaPlayer(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "audio.MTBF031PlayM4aByMediaPlayer"); err != nil {
		return err
	}

	return nil
}

func drive031DUTCheckMediaPlayerStatus(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "audio.MTBF031CheckPlayerStatusAndResume"); err != nil {
		return err
	}

	return nil
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
	defer func() {
		dut.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")

		pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !pause {
			dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/header_play_pause").Do(ctx)
		}
		dut.PressCancelButton(ctx, 3)
	}()

	dut.Client.Comments("Recover env").Do(ctx)
	return nil
}

func MTBF031PlayAudioFromAndroidAndChromeOS(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF031PlayAudioFromAndroidAndChromeOS",
		Description: "ARC++ Playing audio from Android and ChromeOS simultaneously",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Open built-in media player and play")
		if mtbferr := drive031DUTMediaPlayer(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Open VLC player and play")
		if mtbferr := drive031DUTVLCPlayer(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Check media player status")
		if mtbferr := drive031DUTCheckMediaPlayerStatus(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		testing.ContextLog(ctx, "Verify VLC player still play")
		if mtbferr := drive031DUTVerifyVLCPlayer(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
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
