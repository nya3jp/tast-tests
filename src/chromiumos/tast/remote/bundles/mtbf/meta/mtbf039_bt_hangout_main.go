// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/bluetooth"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/services/mtbf/svc"
	uiserv "chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF039BTHangoutMain,
		Desc:         "Switches between HSP and A2DP are functional. A HSP/A2DP device plays music and joins a hangouts call",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"format_mp3.mp3"},
		Vars:         []string{"bluetooth.hangoutsURL", "bluetooth.a2dpDevName", "meta.requestURL", "contact"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_p"},
		Timeout:      time.Minute * 5,
		ServiceDeps: []string{
			"tast.mtbf.bluetooth.Testcases",
			"tast.mtbf.svc.CommService",
			"tast.mtbf.svc.WebService",
			"tast.mtbf.ui.UI",
			"tast.mtbf.multimedia.AudioPlayer",
			"tast.mtbf.ui.KeyboardService",
		},
	})
}

const joinBtn = `document.querySelector("#yDmH0d > div.WOi1Wb > div.GhN39b > div > div > div > div > div > span")`

func drive039CompanionPhone(ctx context.Context, dut *utils.Device, contact, hangoutsURL string) error {
	// if the previous tests fails, the remaining call may block the case
	// relaunch Hangouts app
	testing.ContextLog(ctx, "Unlock phone and relaunch Hangouts app")
	dut.UnlockPhone(ctx)
	dut.Client.ExecAdbCommand(dut.DeviceID, "shell am force-stop com.google.android.talk").Do(ctx)

	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		".SigningInActivity",
		"com.google.android.talk").Do(ctx); err != nil {
		return err
	}
	isNext, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=SKIP::ID=com.google.android.talk:id/promo_button_no", 3000, ui.ObjEventTypeAppear).Do(ctx)
	if isNext {
		dut.Client.UIAClick(dut.DeviceID).Selector("text=SKIP::ID=com.google.android.talk:id/promo_button_no").Do(ctx)
	}

	dut.AllowPermission(ctx, "ID=com.android.permissioncontroller:id/permission_allow_button")

	enterApp, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.talk:id/title::text=Hangouts", 60000, ui.ObjEventTypeAppear).Do(ctx)
	if !enterApp {
		return mtbferrors.New(mtbferrors.CompHangoutsApp, nil)
	}

	testing.ContextLog(ctx, "Join a hangouts call")
	dut.EnterToConversation(ctx, contact)

	sent, _ := dut.Client.UIAVerify(dut.DeviceID, "text="+hangoutsURL).Do(ctx)
	if !sent.True {
		dut.SendMessageToChromeOS(ctx, hangoutsURL)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("text=" + hangoutsURL).Do(ctx)
	dut.AllowPermission(ctx, "text=Allow")

	ok, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=OK", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if ok {
		return mtbferrors.New(mtbferrors.CanootJoinCall, nil)
	}

	joined, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=JOIN", 60000, ui.ObjEventTypeAppear).Do(ctx)
	if !joined {
		return mtbferrors.New(mtbferrors.WithoutJoinButton, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("text=JOIN").Do(ctx)

	// click "OK" if a dialog shows up
	ok, _ = dut.Client.UIAObjEventWait(dut.DeviceID, "TEXT=OK", 2000, ui.ObjEventTypeAppear).Do(ctx)
	if ok {
		dut.Client.UIAClick(dut.DeviceID).Selector("TEXT=OK").Do(ctx)
	}

	return nil
}

func cleanup039CompanionPhone(ctx context.Context, dut *utils.Device, deviceName string) {
	dut.Client.Comments(deviceName).Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/in_call_main_avatar").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/in_call_hang_up").Do(ctx)
	dut.PressCancelButton(ctx, 4)
}

func MTBF039BTHangoutMain(ctx context.Context, s *testing.State) {
	deviceHostname := s.DUT().GetHostname()
	contact := s.RequiredVar("contact")
	hangoutsURL := s.RequiredVar("bluetooth.hangoutsURL")
	deviceName := s.RequiredVar("bluetooth.a2dpDevName")

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF039BTHangoutMain",
		Description: "Switches between HSP and A2DP are functional. A HSP/A2DP device plays music and joins a hangouts call",
		Timeout:     5 * time.Minute,
	}

	common.AudioFilesPrepare(ctx, s, []string{"format_mp3.mp3"})

	c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
	}
	defer c.Close(ctx)

	web := svc.NewWebServiceClient(c.Conn)
	uiSvc := uiserv.NewUIClient(c.Conn)
	ap := multimedia.NewAudioPlayerClient(c.Conn)
	tcs := bluetooth.NewTestcasesClient(c.Conn)

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		phone := utils.NewDevice(client, common.CompanionID)
		if mtbferr := drive039CompanionPhone(ctx, phone, contact, hangoutsURL); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Part1")
		if _, mtbferr := tcs.RunLocal039Part1(ctx, &bluetooth.Case039Request{
			A2DPDeviceName: deviceName,
			HangoutsURL:    hangoutsURL,
		}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Play music")
		if _, mtberr := ap.OpenInDownloads(ctx, &multimedia.FileRequest{
			Filepath: "audios/format_mp3.mp3",
		}); mtberr != nil {
			common.Fatal(ctx, s, mtberr)
		}
		defer ap.CloseAll(ctx, &empty.Empty{})

		testing.Sleep(ctx, 5*time.Second)

		testing.ContextLog(ctx, "Part2")
		if _, mtbferr := tcs.RunLocal039Part2(ctx, &bluetooth.Case039Request{
			A2DPDeviceName: deviceName,
			HangoutsURL:    hangoutsURL,
		}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "HANGOUTS URL: "+hangoutsURL)
		if _, mtbferr := web.OpenURL(ctx, &svc.OpenURLRequest{Url: hangoutsURL}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "joinBtn: ", joinBtn)
		testing.Sleep(ctx, 3*time.Second)

		testing.ContextLog(ctx, "Click Join")
		if _, mtbferr := uiSvc.ClickElement(ctx, &uiserv.ClickElementRequest{Role: "button", Name: "JOIN HANGOUT"}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		testing.ContextLog(ctx, "Part3")
		if _, mtbferr := tcs.RunLocal039Part3(ctx, &bluetooth.Case039Request{
			A2DPDeviceName: deviceName,
			HangoutsURL:    hangoutsURL,
		}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup039CompanionPhone(ctx, compDev, deviceHostname)
		web.CloseURL(ctx, &svc.CloseURLRequest{Url: "hangouts.google.com"})
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
