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
		Func:     MTBF029GAudioDucking,
		Desc:     "Android notifications should duck the existing playback (ARC++)",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"cats.requestURL",
			"contact",
		},
	})
}

func drive029GDUTAndCompanionPhone(ctx context.Context, dutDev, compDev *utils.Device, contact string) error {
	dutDev.OpenGoogleMusicAndPlay(ctx)
	compDev.SendHangoutsMessage(ctx, contact)
	return nil
}

func cleanup029GDUTAndCompanionPhone(ctx context.Context, dutDev, compDev *utils.Device) {
	compDev.Client.Press(compDev.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	compDev.Client.Press(compDev.DeviceID, ui.OprKeyEventHOME).Do(ctx)

	pause, _ := dutDev.Client.UIAObjEventWait(dutDev.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dutDev.Client.UIAClick(dutDev.DeviceID).Selector("ID=com.google.android.music:id/pause").Do(ctx, service.Sleep(time.Second*2))
	}

	dutDev.PressCancelButton(ctx, 1)
}

func MTBF029GAudioDucking(ctx context.Context, s *testing.State) {
	dutID, err := s.DUT().GetARCDeviceID(ctx)
	if err != nil {
		s.Fatal(mtbferrors.OSNoArcDeviceID, err)
	}

	addr, err := common.CatsNodeAddress(ctx, s)
	if err != nil {
		s.Fatal("Failed to get cats node addr: ", err)
	}

	contact, ok := s.Var("contact")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "contact"))
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
		compDev := utils.NewDevice(client, common.CompanionID)

		if err := drive029GDUTAndCompanionPhone(ctx, dutDev, compDev, contact); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)
		compDev := utils.NewDevice(client, common.CompanionID)

		cleanup029GDUTAndCompanionPhone(ctx, dutDev, compDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
