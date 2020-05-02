// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF009GAudioFilesPlaying,
		Desc:     "ARC++ plays local files from vlc and verifies audio volume level is changed based on volume controls",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func MTBF009GAudioFilesPlaying(ctx context.Context, s *testing.State) {
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

		if err := play009GMedia(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive009GDUT(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanup009GVlcPlayer(ctx, dutDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}

// cleanup009GVlcPlayer cleans up vlc app.
func cleanup009GVlcPlayer(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	isPause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isPause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/header_play_pause").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
}

// drive009GDUT runs a tast case of MTBF009AdjustVolume.
func drive009GDUT(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "audio.MTBF009AdjustVolume"); err != nil {
		return err
	}

	return nil
}

// play009GMedia opens vlc and play media files.
func play009GMedia(ctx context.Context, dut *utils.Device) error {
	dut.OpenVLCAndEnterToDownload(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=audios").Do(ctx)
	// Play four types of audio separately and verify whether the songs is playing.
	fileTypes := []string{"m4a", "mp3", "ogg", "wav"}
	var failTypes []string

	for _, filetype := range fileTypes {
		hasAudio, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "textcontains="+filetype, 20000, ui.ObjEventTypeAppear).Do(ctx)
		if !hasAudio {
			return errors.Errorf("can't find the target %s audio file. user errcode='7002'", filetype)
		}
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::textcontains=" + filetype).Do(ctx)
		// If "Got it, dismiss it" button appear, click it.
		utils.ClickSelector(ctx, dut, "textstartwith=Got it")
		// Verify the playing audio file name is as expected.
		filename, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/title::textcontains="+filetype, 6000, ui.ObjEventTypeAppear).Do(ctx)
		// Verify the play_pause_button status is 'Play'.
		isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !(filename && isPlay) {
			failTypes = append(failTypes, filetype)
		}
		// send pause key
		dut.Client.ExecCommand(dut.DeviceID, "shell input keyevent KEYCODE_MEDIA_PAUSE").Do(ctx)
	}

	if len(failTypes) > 0 {
		return mtbferrors.New(mtbferrors.VLCAppNotPlay, nil)
	}
	return nil
}
