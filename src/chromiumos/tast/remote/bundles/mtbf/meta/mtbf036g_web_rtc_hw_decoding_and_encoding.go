// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"crypto/rand"
	"encoding/hex"

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
		Func:     MTBF036GWebRTCHWDecodingAndEncoding,
		Desc:     "VP8 HW decoding/encoding simultaneously works with WebRTC video chat",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

func randomRoomName() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func drive036GCompanionPhone(ctx context.Context, dut *utils.Device, roomName string) error {
	dut.Client.ExecCommand(dut.DeviceID, "shell am start -n com.android.chrome/com.google.android.apps.chrome.Main").Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventHOME).Do(ctx)
	dut.Client.ExecCommand(dut.DeviceID, "shell am start -n com.android.chrome/com.google.android.apps.chrome.Main").Do(ctx)
	accept, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.android.chrome:id/terms_accept", 1000, ui.ObjEventTypeAppear).Do(ctx)
	if accept {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.android.chrome:id/terms_accept").Do(ctx)
	}

	thank, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=No thanks", 1000, ui.ObjEventTypeAppear).Do(ctx)
	if thank {
		dut.Client.UIAClick(dut.DeviceID).Selector("text=No thanks").Do(ctx)
	}

	box, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.android.chrome:id/search_box_text", 1000, ui.ObjEventTypeAppear).Do(ctx)
	if box {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.android.chrome:id/search_box_text").Do(ctx)
	} else {
		urlBar, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.android.chrome:id/url_bar", 1000, ui.ObjEventTypeAppear).Do(ctx)
		if urlBar {
			dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.android.chrome:id/url_bar").Do(ctx)
		} else {
			return mtbferrors.New(mtbferrors.FoundSearchBox, nil)
		}
	}

	roomURL := "https://appr.tc/r/" + roomName

	dut.Client.InputText(dut.DeviceID, roomURL).Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventENTER).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=JOIN").Do(ctx)

	dut.AllowPermission(ctx, "text=Allow")

	return nil
}

func drive036GDUT(ctx context.Context, s *testing.State, roomName string) error {
	flags := common.GetFlags(s)
	flags = append(flags, "-var=dynamic.var="+roomName)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "mtbfutil.JoinAppRtcRoom"); err != nil {
		return err
	}

	return nil
}

func cleanup036GCompanionPhone(ctx context.Context, dut *utils.Device) {
	dut.JoinAppRtcCleanup(ctx)
}

func MTBF036GWebRTCHWDecodingAndEncoding(ctx context.Context, s *testing.State) {
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

	roomName, err := randomRoomName()
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoAppRtcRoomName, err))
	}

	report, _, err := androidTest.RunDelegate(ctx, sdk.CaseDescription{
		Name:        "case_name",
		Description: "A new case",
		ReportPath:  "report/path",
		DutID:       dutID,
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		if err := drive036GCompanionPhone(ctx, compDev, roomName); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive036GDUT(ctx, s, roomName); err != nil {
			utils.FailCase(ctx, client, err)
		}
		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		cleanup036GCompanionPhone(ctx, compDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
