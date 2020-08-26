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
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF036WebRTCHWDecodingAndEncoding,
		Desc:     "VP8 HW decoding/encoding simultaneously works with WebRTC video chat",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.requestURL"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
			"tast.mtbf.svc.WebService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
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

func drive036CompanionPhone(ctx context.Context, dut *utils.Device, roomName string) error {
	testing.ContextLog(ctx, "Start companion phone actions")

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

	testing.ContextLog(ctx, "Open AppRTC chatroom on companion phone")
	roomURL := "https://appr.tc/r/" + roomName

	dut.InputText(ctx, roomURL)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventENTER).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("text=JOIN").Do(ctx)

	dut.AllowPermission(ctx, "text=Allow")

	return nil
}

func cleanup036CompanionPhone(ctx context.Context, dut *utils.Device) {
	dut.JoinAppRtcCleanup(ctx)
}

func MTBF036WebRTCHWDecodingAndEncoding(ctx context.Context, s *testing.State) {
	roomName, err := randomRoomName()
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.VideoAppRtcRoomName, err))
	}

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF036WebRTCHWDecodingAndEncoding",
		Description: "VP8 HW decoding/encoding simultaneously works with WebRTC video chat",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		if mtbferr := drive036CompanionPhone(ctx, compDev, roomName); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Open AppRTC chatroom on DUT")

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		ws := svc.NewWebServiceClient(cl.Conn)

		if _, mtbferr := ws.JoinAppRTCRoom(ctx, &svc.JoinAppRTCRoomRequest{RoomName: roomName}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		testing.ContextLog(ctx, "Start case cleanup")
		compDev := utils.NewDevice(client, common.CompanionID)

		cleanup036CompanionPhone(ctx, compDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
