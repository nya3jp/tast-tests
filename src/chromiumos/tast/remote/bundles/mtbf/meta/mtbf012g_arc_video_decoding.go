// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

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
		Func:     MTBF012GARCVideoDecoding,
		Desc:     "ARC++ Test H264 H/W decoding",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func drive012GDUT(ctx context.Context, dut *utils.Device) error {
	dut.OpenVLCAndEnterToDownload(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=videos").Do(ctx)
	video, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=test_video.mp4", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !video {
		return mtbferrors.New(mtbferrors.FoundVideoFile, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("text=test_video.mp4").Snapshot(false).Do(ctx, service.Sleep(0))
	dut.Client.UIASwipe(dut.DeviceID, 1275, 512, 548, 512, 10).Do(ctx)
	return nil
}

func cleanup012DUT(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")
	dut.PressCancelButton(ctx, 5)
}

func MTBF012GARCVideoDecoding(ctx context.Context, s *testing.State) {
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

		if err := drive012GDUT(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}
		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanup012DUT(ctx, dutDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
