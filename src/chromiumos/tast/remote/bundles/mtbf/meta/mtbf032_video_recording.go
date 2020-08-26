// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
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
		Func:     MTBF032VideoRecording,
		Desc:     "ARC++ Test camera video recording and playback",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.requestURL"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

func drive032DUT(ctx context.Context, dut *utils.Device) error {
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.android.camera.CameraLauncher",
		"com.google.android.GoogleCameraArc",
	).Do(ctx, service.Sleep(time.Second*2)); err != nil {
		return err
	}

	//dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "text=Close app")
	//dut.Client.UIAClick(dut.DeviceID).Selector("ID=android:id/button1::text=OK").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "ID=android:id/button1::text=OK")
	//dut.Client.UIAClick(dut.DeviceID).Selector("text=ALLOW").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "text=ALLOW")

	enterApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "packagename=com.google.android.GoogleCameraArc", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !enterApp {
		return mtbferrors.New(mtbferrors.EnterCameraApp, nil)
	}

	video, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Start Recording", 24000, ui.ObjEventTypeAppear).Do(ctx)
	if !video {
		btn, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/video_switch_button", 12000, ui.ObjEventTypeAppear).Do(ctx)
		if btn {
			dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/video_switch_button").Do(ctx)
		} else {
			return mtbferrors.New(mtbferrors.VideoSwitchButton, nil)
		}
	}

	dut.ClickShutterButton(ctx)
	dut.Client.Delay(5000).Do(ctx)
	dut.ClickShutterButton(ctx)

	stop, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Start Recording", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !stop {
		dut.ClickShutterButton(ctx)
	}

	dut.OpenCapturedFrameOrRecordedVideo(ctx)
	dut.PressCancelButton(ctx, 1)

	camera, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/camera_switch_button::clickable=True", 2000, ui.ObjEventTypeAppear).Do(ctx)
	dut.Client.Comments(fmt.Sprintf("has switch camera: %t", camera)).Do(ctx)

	if !camera {
		return nil
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/camera_switch_button").Do(ctx)

	normal, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if !normal {
		return mtbferrors.New(mtbferrors.CameraAppCrash, nil)
	}

	dut.ClickShutterButton(ctx)
	dut.Client.Delay(5000).Do(ctx)
	dut.ClickShutterButton(ctx)

	stop, _ = dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Start Recording", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !stop {
		dut.ClickShutterButton(ctx)
	}

	dut.OpenCapturedFrameOrRecordedVideo(ctx)

	return nil
}

func cleanup032DUT(ctx context.Context, dut *utils.Device) {
	dut.DeleteCreatedFrameOrVideo(ctx, 3)
	dut.PressCancelButton(ctx, 3)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=DISMISS").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx)
}

func MTBF032VideoRecording(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF032VideoRecording",
		Description: "ARC++ Test camera video recording and playback",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := drive032DUT(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cleanup032DUT(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
