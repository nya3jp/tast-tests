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
		Func:     MTBF048GSendRecordedVideoToHangouts,
		Desc:     "ARC++ Video Intent test with hangout recording and playing video",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive048GDUT(ctx context.Context, dut *utils.Device) error {
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		".SigningInActivity",
		"com.google.android.talk").Do(ctx, service.Sleep(time.Second*2)); err != nil {
		return err
	}

	isNext, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=SKIP::ID=com.google.android.talk:id/promo_button_no", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if isNext {
		dut.Client.UIAClick(dut.DeviceID).Selector("text=SKIP::ID=com.google.android.talk:id/promo_button_no").Do(ctx)
	}

	dut.AllowPermission(ctx, "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button")

	enterApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.talk:id/title::text=Hangouts", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !enterApp {
		return mtbferrors.New(mtbferrors.HangoutsApp, nil)
	}

	dut.EnterToConversationAndSelectVideo(ctx)
	dut.RecordVideoAndPlay(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=Retake").Do(ctx)
	dut.RecordVideoAndPlay(ctx)

	stop, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=VLC keeps stopping", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if stop {
		return mtbferrors.New(mtbferrors.VLCKeepStop, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/shutter_button").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/preview_media_send_button").Do(ctx)

	dut.WaitForGCAScreen(ctx, ui.ObjEventTypeDisappear)
	dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.talk:id/footer_text::desc=Now", 10000, ui.ObjEventTypeAppear).Do(ctx)

	return nil
}

func cleanup048GDUT(ctx context.Context, dut *utils.Device) {
	dut.Client.UIAClick(dut.DeviceID).Selector(("text=DISMISS")).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector(("text=Close app")).Do(ctx)
	dut.PressCancelButton(ctx, 1)
	dut.PressCancelButton(ctx, 1)
	dut.PressCancelButton(ctx, 4)
}

func MTBF048GSendRecordedVideoToHangouts(ctx context.Context, s *testing.State) {
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

		if err := drive048GDUT(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}
		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanup048GDUT(ctx, dutDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
