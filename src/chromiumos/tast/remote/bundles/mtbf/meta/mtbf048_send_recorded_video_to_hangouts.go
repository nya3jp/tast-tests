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
		Func:     MTBF048SendRecordedVideoToHangouts,
		Desc:     "ARC++ Video Intent test with hangout recording and playing video",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.requestURL"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

func drive048DUT(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Open Hangouts Talk app")
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

	testing.ContextLog(ctx, "Enter conversation and select video")
	dut.EnterToConversationAndSelectVideo(ctx)

	testing.ContextLog(ctx, "Start record video and play")

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.camera.hd.vip:id/take_photo").Do(ctx)
	dut.Client.Delay(5000).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.camera.hd.vip:id/take_photo").Do(ctx)
	dut.Client.Delay(5000).Do(ctx)
	testing.ContextLog(ctx, "Cancel sending video")
	dut.PressCancelButton(ctx, 1)

	testing.ContextLog(ctx, "Start record video and play")
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/video_camerapicker_indicator_icon").Do(ctx)
	dut.AllowPermission(ctx, "text=ALLOW::ID=com.android.packageinstaller:id/permission_allow_button")
	dut.WaitForGCAScreen(ctx, ui.ObjEventTypeAppear)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.camera.hd.vip:id/take_photo").Do(ctx)
	dut.Client.Delay(5000).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.camera.hd.vip:id/take_photo").Do(ctx)
	dut.Client.Delay(5000).Do(ctx)
	testing.ContextLog(ctx, "Sending video")
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/preview_media_send_button").Do(ctx)

	dut.WaitForGCAScreen(ctx, ui.ObjEventTypeDisappear)
	dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.talk:id/footer_text::desc=Now", 10000, ui.ObjEventTypeAppear).Do(ctx)

	return nil
}

func cleanup048DUT(ctx context.Context, dut *utils.Device) {
	dut.Client.UIAClick(dut.DeviceID).Selector(("text=DISMISS")).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector(("text=Close app")).Do(ctx)
	dut.PressCancelButton(ctx, 1)
	dut.PressCancelButton(ctx, 1)
	dut.PressCancelButton(ctx, 4)
}

func MTBF048SendRecordedVideoToHangouts(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF048SendRecordedVideoToHangouts",
		Description: "ARC++ Video Intent test with hangout recording and playing video",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := drive048DUT(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup048DUT(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
