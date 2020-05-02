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
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF027GNotificationsDuckExistingPlayback,
		Desc:     "Short playbacks/notifications should duck the existing playback",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"cats.requestURL",
			"contact",
		},
	})
}

// MTBF027GNotificationsDuckExistingPlayback Run CATS case
// Procedure:
// 1. Start playing audio/video from browser or default player Ex: YouTube video.
// 2. Play any short (<5 seconds) ducking audio, ex: http://rebeccahughes.github.io/media/audio-focus/transient_duck.html
// 3. Observe behavior.
// 4. Let YouTube video play and open Gmail in another page.
// 5. Send chat message from different device to this Gmail account to get notification (make sure notification is enabled from Gmail).
// 6. Observe video behavior.
func MTBF027GNotificationsDuckExistingPlayback(ctx context.Context, s *testing.State) {
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
		compDev := utils.NewDevice(client, common.CompanionID)

		if err := common.DriveDUT(ctx, s, "video.MTBF027ANotificationsDuckExistingPlayback"); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := compDev.SendHangoutsMessage(ctx, contact); err != nil {
			utils.FailCase(ctx, client, err)
		}
		if err := common.DriveDUT(ctx, s, "video.MTBF027BNotificationsDuckExistingPlayback"); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		cleanup027GDUTAndCompanionPhone(ctx, compDev)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("Test failed: ", err)
	}
}

func cleanup027GDUTAndCompanionPhone(ctx context.Context, dut *utils.Device) {
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventHOME).Do(ctx)
}
