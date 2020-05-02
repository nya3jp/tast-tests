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
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF010GAudioRecording,
		Desc:     "ARC++ Audio recording and playback",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive010GDUT(ctx context.Context, dut *utils.Device) error {
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		".MainActivity",
		"com.media.bestrecorder.audiorecorder").Do(ctx, service.Sleep(time.Second*2)); err != nil {
		return err
	}

	//dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx, service.Suppress())
	utils.ClickSelector(ctx, dut, "text=Close app")

	thank, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=No, thanks", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if thank {
		dut.PressCancelButton(ctx, 1)
	}

	//dut.Client.UIAClick(dut.DeviceID).Selector("text=No").Do(ctx, service.Suppress())
	utils.ClickSelector(ctx, dut, "text=No")

	enter, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=Voice Recorder", 6000, ui.ObjEventTypeAppear).FailOnNotMatch(true).Do(ctx)
	if !enter {
		return mtbferrors.New(mtbferrors.VoiceRecordApp, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_record_start").Do(ctx, service.Sleep(time.Second*25))

	start, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.media.bestrecorder.audiorecorder:id/layout_pause_record", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !start {
		return mtbferrors.New(mtbferrors.StartRecord, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_record_start").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=OK").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.media.bestrecorder.audiorecorder:id/btn_play_current_record").Do(ctx, service.Sleep(0))

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

func cleanup010GDUT(ctx context.Context, dut *utils.Device) {
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

func MTBF010GAudioRecording(ctx context.Context, s *testing.State) {
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

		if err := drive010GDUT(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}
		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanup010GDUT(ctx, dutDev)
		return nil, nil
	})

	_ = report
	//s.Fatal(report)

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
