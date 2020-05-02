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
		Func:     MTBF031GPlayAudioFromAndroidAndChromeOS,
		Desc:     "ARC++ Playing audio from Android and ChromeOS simultaneously",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive031GDUTMediaPlayer(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "audio.MTBF031PlayM4aByMediaPlayer"); err != nil {
		return err
	}

	return nil
}

func drive031GDUTCheckMediaPlayerStatus(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "audio.MTBF031CheckPlayerStatusAndResume"); err != nil {
		return err
	}

	return nil
}

func drive031GDUTVLCPlayer(ctx context.Context, dut *utils.Device) error {
	dut.OpenVLCAndEnterToDownload(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=audios").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::textcontains=mp3").Do(ctx)
	//dut.Client.UIAClick(dut.DeviceID).Selector("textstartwith=Got it").Do(ctx, service.Suppress())
	utils.ClickSelector(ctx, dut, "textstartwith=Got it")

	play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !play {
		return mtbferrors.New(mtbferrors.VLCAppNotPlay, nil)
	}

	return nil
}

func drive031GDUTVerifyVLCPlayer(ctx context.Context, dut *utils.Device) error {
	dut.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")

	pause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		return mtbferrors.New(mtbferrors.VLCAppNotPause, nil)
	}

	return nil
}

func drive031GDUTCleanup(ctx context.Context, dut *utils.Device) error {
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

func MTBF031GPlayAudioFromAndroidAndChromeOS(ctx context.Context, s *testing.State) {
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

	report, _, err := androidTest.RunDelegate(ctx, sdk.CaseDescription{
		Name:        "case_name",
		Description: "A new case",
		ReportPath:  "report/path",
		DutID:       dutID,
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		if err := drive031GDUTMediaPlayer(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive031GDUTVLCPlayer(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive031GDUTCheckMediaPlayerStatus(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive031GDUTVerifyVLCPlayer(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
