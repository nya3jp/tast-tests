// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

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
		Func:     MTBF039BTHangoutMain,
		Desc:     "Switches between HSP and A2DP are functional. A HSP/A2DP device plays music and joins a hangouts call",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"video.hangoutsURL", "cats.requestURL", "contact"},
	})
}

func drive039CompanionPhone(ctx context.Context, dut *utils.Device, contact, hangoutsURL string) error {
	defer func() {
		ok, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "TEXT=OK", 2000, ui.ObjEventTypeAppear).Do(ctx)
		if ok {
			dut.Client.UIAClick(dut.DeviceID).Selector("TEXT=OK").Do(ctx)
		}
	}()

	testing.ContextLog(ctx, "Unlock phone and quit joined call")
	dut.UnlockPhone(ctx)
	dut.QuitJoinedCall(ctx)

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

	return nil
}

func drive039DUT(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "bluetooth.MTBF039MusicHangout"); err != nil {
		return err
	}

	return nil
}

func cleanup039CompanionPhone(ctx context.Context, dut *utils.Device, deviceName string) {
	dut.Client.Comments(deviceName).Do(ctx)
	//dut.QuitJoinedCall(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/in_call_main_avatar").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.talk:id/in_call_hang_up").Do(ctx)
	dut.PressCancelButton(ctx, 4)
}

func MTBF039BTHangoutMain(ctx context.Context, s *testing.State) {
	deviceHostname := s.DUT().GetHostname()
	contact, ok := s.Var("contact")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "contact"))
	}

	hangoutsURL, ok := s.Var("video.hangoutsURL")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "video.hangoutsURL"))
	}

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF039BTHangoutMain",
		Description: "Switches between HSP and A2DP are functional. A HSP/A2DP device plays music and joins a hangouts call",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)
		compDev.QuitJoinedCall(ctx)

		if mtbferr := drive039CompanionPhone(ctx, compDev, contact, hangoutsURL); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := drive039DUT(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup039CompanionPhone(ctx, compDev, deviceHostname)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
