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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF010AudioRecording,
		Desc:     "ARC++ Audio recording and playback",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.requestURL"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

func drive010DUT(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Open audio recorder app")
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		".MainActivity",
		"com.media.bestrecorder.audiorecorder").Do(ctx, service.Sleep(time.Second*2)); err != nil {
		return err
	}

	//dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "text=Close app")

	thank, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=No, thanks", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if thank {
		dut.PressCancelButton(ctx, 1)
	}

	//dut.Client.UIAClick(dut.DeviceID).Selector("text=No").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "text=No")

	listTab, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "textstartwith=List", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if listTab {
		dut.PressCancelButton(ctx, 1)
	}

	enter, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=Voice Recorder", 6000, ui.ObjEventTypeAppear).FailOnNotMatch(true).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.VoiceRecordApp, nil)
	}

	testing.ContextLog(ctx, "Start recording")
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_record_start").Do(ctx, service.Sleep(time.Second*25))

	start, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.media.bestrecorder.audiorecorder:id/layout_pause_record", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !start {
		return mtbferrors.New(mtbferrors.StartRecord, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_record_start").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=OK").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_play_current_record").Do(ctx, service.Sleep(0))

	testing.ContextLog(ctx, "Display recorded information")
	total, _ := dut.Client.GetWidgetText2(dut.DeviceID, "ID=com.media.bestrecorder.audiorecorder:id/fragment_detail_duration").Snapshot(false).Do(ctx, service.Sleep(0))
	player, _ := dut.Client.GetWidgetText2(dut.DeviceID, "ID=com.media.bestrecorder.audiorecorder:id/duration_time").Snapshot(false).Do(ctx, service.Sleep(0))

	if total == "" || player == "" {
		return mtbferrors.New(mtbferrors.CannotVerifyRecord, nil)
	}

	if total == player {
		play, _ := dut.Client.FindUIObjectOnDevice(dut.DeviceID, "query_image/play_recorded_audio.png").MatchRate(0.5).Do(ctx)
		if !play.Matched {
			return mtbferrors.New(mtbferrors.RecordedAudioNotPlay, nil)
		}
	} else {
		return mtbferrors.New(mtbferrors.RecordTimeNotMatch, nil)
	}

	return nil
}

func cleanup010DUT(ctx context.Context, dut *utils.Device) {
	dut.DeleteCreatedAudioFile(ctx)
	dut.PressCancelButton(ctx, 2)
	thank, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=No, thanks", 2000, ui.ObjEventTypeAppear).Do(ctx)

	if thank {
		dut.Client.UIAClick(dut.DeviceID).Selector("text=No, thanks").Do(ctx)
		dut.PressCancelButton(ctx, 1)
	}

	main, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=Voice Recorder", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if main {
		dut.PressCancelButton(ctx, 1)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("text=Yes").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=No, thanks").Do(ctx)
}

func MTBF010AudioRecording(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF010AudioRecording",
		Description: "ARC++ Audio recording and playback",
		Timeout:     5 * time.Minute,
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := drive010DUT(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup010DUT(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
