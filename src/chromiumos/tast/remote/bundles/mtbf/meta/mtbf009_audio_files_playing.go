// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"strings"

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
		Func:     MTBF009AudioFilesPlaying,
		Desc:     "ARC++ plays local files from vlc and verifies audio volume level is changed based on volume controls",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func MTBF009AudioFilesPlaying(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF009AudioFilesPlaying",
		Description: "ARC++ plays local files from vlc and verifies audio volume level is changed based on volume controls",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := play009Media(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := drive009DUT(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cleanup009VlcPlayer(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

// cleanup009VlcPlayer cleans up vlc app.
func cleanup009VlcPlayer(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	isPause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isPause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/header_play_pause").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
}

// drive009DUT runs a tast case of MTBF009AdjustVolume.
func drive009DUT(ctx context.Context, s *testing.State) error {
	testing.ContextLog(ctx, "Starting to run adjust volume sub case and verify")
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "audio.MTBF009AdjustVolume"); err != nil {
		return err
	}

	return nil
}

// play009Media opens vlc and play media files.
func play009Media(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Starting to run media files")
	dut.OpenVLCAndEnterToDownload(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=audios").Do(ctx)
	// Play four types of audio separately and verify whether the songs is playing.
	fileTypes := []string{"m4a", "mp3", "ogg", "wav"}
	var failTypes []string

	for _, filetype := range fileTypes {
		testing.ContextLogf(ctx, "Playing media file(%s)", filetype)
		hasAudio, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "textcontains="+filetype, 20000, ui.ObjEventTypeAppear).Do(ctx)
		if !hasAudio {
			return mtbferrors.New(mtbferrors.FoundAudioFile, nil, filetype)
		}
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::textcontains=" + filetype).Do(ctx)
		// If "Got it, dismiss it" button appear, click it.
		dut.ClickSelector(ctx, "textstartwith=Got it")
		// Verify the playing audio file name is as expected.
		filename, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/title::textcontains="+filetype, 6000, ui.ObjEventTypeAppear).Do(ctx)
		// Verify the play_pause_button status is 'Play'.
		testing.ContextLog(ctx, "Verify player is playing")
		isPlay, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !(filename && isPlay) {
			testing.ContextLogf(ctx, "Verify player playing %s failed", filename)
			failTypes = append(failTypes, filetype)
		}
		// send pause key
		dut.Client.ExecCommand(dut.DeviceID, "shell input keyevent KEYCODE_MEDIA_PAUSE").Do(ctx)
	}

	if len(failTypes) > 0 {
		return mtbferrors.New(mtbferrors.VLCAppNotPlayFileTypes, nil, strings.Join(failTypes, ","))
	}
	return nil
}
