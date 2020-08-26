// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"strings"
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
		Func:         MTBF009AudioFilesPlaying,
		Desc:         "ARC++ plays local files from vlc and verifies audio volume level is changed based on volume controls",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"format_m4a.m4a", "format_mp3.mp3", "format_ogg.ogg", "format_wav.wav"},
		Vars:         []string{"meta.requestURL"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_p"},
		ServiceDeps: []string{
			"tast.mtbf.multimedia.VolumeService",
			"tast.mtbf.svc.CommService",
		},
	})
}

func MTBF009AudioFilesPlaying(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF009AudioFilesPlaying",
		Description: "ARC++ plays local files from vlc and verifies audio volume level is changed based on volume controls",
		Timeout:     5 * time.Minute,
	}

	// prepare audio files
	common.AudioFilesPrepare(ctx, s, []string{"format_m4a.m4a", "format_mp3.mp3", "format_ogg.ogg", "format_wav.wav"})

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer c.Close(ctx)

		volume := multimedia.NewVolumeServiceClient(c.Conn)
		if err = setDefaultVolume(ctx, s, volume); err != nil {
			common.Fatal(ctx, s, err)
		}
		defer setDefaultVolume(ctx, s, volume)

		if mtbferr := play009Media(ctx, dutDev, volume); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
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

func setDefaultVolume(ctx context.Context, s *testing.State, volume multimedia.VolumeServiceClient) error {
	testing.ContextLog(ctx, "Unmute DUT")
	if _, err := volume.Unmute(ctx, &empty.Empty{}); err != nil {
		return err
	}
	testing.Sleep(ctx, 3*time.Second)

	s.Log("Set volume to 50")
	_, err := volume.Set(ctx, &multimedia.VolumeRequest{Value: 50, Check: true})
	return err
}

// cleanup009VlcPlayer cleans up vlc app.
func cleanup009VlcPlayer(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	isPause, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !isPause {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/header_play_pause").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
	dut.ClickSelector(ctx, "text=OK")
	dut.PressCancelButton(ctx, 1)
	dut.ClickSelector(ctx, "text=OK")
}

// play009Media opens vlc and play media files.
func play009Media(ctx context.Context, dut *utils.Device, volume multimedia.VolumeServiceClient) error {
	testing.ContextLog(ctx, "Starting to run media files")
	dut.OpenVLCAndEnterToDownload(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=audios").Do(ctx)

	var (
		fileTypes = map[string]struct{}{
			"m4a": {},
			"mp3": {},
			"ogg": {},
			"wav": {},
		}
		failedTypes []string
	)

	// Play four types of audio separately and verify whether the songs is playing.
	for filetype := range fileTypes {
		testing.ContextLogf(ctx, "Playing media file(%s)", filetype)
		found, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "textcontains="+filetype, 20000, ui.ObjEventTypeAppear).Do(ctx)
		if !found {
			return mtbferrors.New(mtbferrors.FoundAudioFile, nil, filetype)
		}
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::textcontains=" + filetype).Do(ctx)

		// if "Got it, dismiss it" button appear, click it.
		dut.ClickSelector(ctx, "textstartwith=Got it")

		// verify the playing audio file name is as expected.
		filename, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/title::textcontains="+filetype, 6000, ui.ObjEventTypeAppear).Do(ctx)

		// Verify the play_pause_button status is 'Play'.
		testing.ContextLog(ctx, "Verify player is playing")
		playing, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=org.videolan.vlc:id/header_play_pause::desc=Pause", 6000, ui.ObjEventTypeAppear).Do(ctx)
		if !(filename && playing) {
			testing.ContextLogf(ctx, "Verify player playing %s failed", filename)
			failedTypes = append(failedTypes, filetype)
		}

		// change volume by pressing F10(louder), F9(smaller), F8(mute)
		for _, key := range []multimedia.FnKey{multimedia.FnKey_F10, multimedia.FnKey_F9, multimedia.FnKey_F8} {
			testing.ContextLog(ctx, "Press key ", multimedia.FnKey_name[int32(key)])
			if _, err := volume.PressKey(ctx, &multimedia.PressKeyRequest{Key: key, Check: true}); err != nil {
				return err
			}
			testing.Sleep(ctx, 2*time.Second)
		}

		testing.ContextLog(ctx, "Check if volume is mute")
		v, err := volume.Get(ctx, &empty.Empty{})
		if err != nil {
			return err
		}
		if !v.Mute {
			return mtbferrors.New(mtbferrors.AudioMute, nil)
		}

		testing.ContextLog(ctx, "Unmute DUT")
		if _, err = volume.Unmute(ctx, &empty.Empty{}); err != nil {
			return err
		}

		// send pause key
		dut.Client.ExecCommand(dut.DeviceID, "shell input keyevent KEYCODE_MEDIA_PAUSE").Do(ctx)
	}

	if len(failedTypes) > 0 {
		return mtbferrors.New(mtbferrors.VLCAppNotPlayFileTypes, nil, strings.Join(failedTypes, ","))
	}
	return nil
}
