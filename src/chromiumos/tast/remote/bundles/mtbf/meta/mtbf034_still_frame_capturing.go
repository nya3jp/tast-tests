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
		Func:     MTBF034StillFrameCapturing,
		Desc:     "ARC++ Test camera still capture",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.requestURL"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

func drive034DUT(ctx context.Context, dut *utils.Device) error {
	// Enter to app
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.android.camera.CameraLauncher",
		"com.google.android.GoogleCameraArc").Do(ctx, service.Sleep(time.Second*2)); err != nil {
		return nil
	}
	//dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "text=Close app")

	// Init app. Click OK
	//dut.Client.UIAClick(dut.DeviceID).Selector("ID=android:id/button1::text=OK").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "ID=android:id/button1::text=OK")
	//dut.Client.UIAClick(dut.DeviceID).Selector("text=ALLOW").Do(ctx, service.Suppress())
	dut.ClickSelector(ctx, "text=ALLOW")

	// Verify whether enter to Camera app
	enterApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "packagename=com.google.android.GoogleCameraArc", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !enterApp {
		return mtbferrors.New(mtbferrors.EnterCameraApp, nil)
	}

	// Verify whethr in taking photo page. If not, Switch to photo page.
	phone, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button::desc=Shutter", 120000, ui.ObjEventTypeAppear).Do(ctx)
	if !phone {
		btn, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/photo_switch_button", 12000, ui.ObjEventTypeAppear).Do(ctx)
		if btn {
			//dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/photo_switch_button").Do(ctx, service.Suppress())
			dut.ClickSelector(ctx, "ID=com.google.android.GoogleCameraArc:id/photo_switch_button")
		} else {
			return mtbferrors.New(mtbferrors.PhotoButton, nil)
		}
	}

	// Capture still frame.
	dut.ClickShutterButton(ctx)
	// Open captured frame to check.
	dut.OpenCapturedFrameOrRecordedVideo(ctx)

	// Open frame info page and check resolution
	// Click info button.
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=Details").Do(ctx)
	width, _ := dut.Client.GetWidgetText2(dut.DeviceID, "ID=android:id/text1::textstartwith=Width:").Do(ctx)
	height, _ := dut.Client.GetWidgetText2(dut.DeviceID, "ID=android:id/text1::textstartwith=Height:").Do(ctx)

	if width == "" || height == "" {
		return mtbferrors.New(mtbferrors.WithoutResolution, nil)
	}
	dut.Client.Comments(fmt.Sprintf(`Resolution is: %s * %s.`, width[7:], height[8:])).Do(ctx)
	dut.PressCancelButton(ctx, 2)

	// Switch camera
	camera, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/camera_switch_button::clickable=True", 2000, ui.ObjEventTypeAppear).Do(ctx)
	dut.Client.Comments(fmt.Sprintf(`has switch camera: %t `, camera)).Do(ctx)

	if !camera {
		return nil
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.GoogleCameraArc:id/camera_switch_button").Do(ctx)
	normal, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.GoogleCameraArc:id/shutter_button", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if !normal {
		return mtbferrors.New(mtbferrors.CameraAppCrash, nil)
	}

	// Capture still frame.
	dut.ClickShutterButton(ctx)
	// Open captured frame to check.
	dut.OpenCapturedFrameOrRecordedVideo(ctx)
	// Open frame info page and check resolution
	// Click info button.
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=Details").Do(ctx)

	width, _ = dut.Client.GetWidgetText2(dut.DeviceID, "ID=android:id/text1::textstartwith=Width:").Do(ctx)
	height, _ = dut.Client.GetWidgetText2(dut.DeviceID, "ID=android:id/text1::textstartwith=Height:").Do(ctx)

	if width == "" || height == "" {
		return mtbferrors.New(mtbferrors.WithoutResolution, nil)
	}
	dut.Client.Comments(fmt.Sprintf(`Resolution is: %s * %s.`, width, height)).Do(ctx)
	dut.PressCancelButton(ctx, 1)

	return nil
}

func cleanup034DUT(ctx context.Context, dut *utils.Device) {
	dut.DeleteCreatedFrameOrVideo(ctx, 3)
	dut.PressCancelButton(ctx, 3)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=DISMISS").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=Close app").Do(ctx)
}

func MTBF034StillFrameCapturing(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF034StillFrameCapturing",
		Description: "ARC++ Test camera still capture",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := drive034DUT(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup034DUT(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
