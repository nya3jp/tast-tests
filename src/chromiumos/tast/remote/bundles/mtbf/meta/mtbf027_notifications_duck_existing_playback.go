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
		Func:     MTBF027NotificationsDuckExistingPlayback,
		Desc:     "Short playbacks/notifications should duck the existing playback",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars: []string{
			"cats.requestURL",
			"contact",
		},
	})
}

// MTBF027NotificationsDuckExistingPlayback Run CATS case
// Procedure:
// 1. Start playing audio/video from browser or default player Ex: YouTube video.
// 2. Play any short (<5 seconds) ducking audio, ex: http://rebeccahughes.github.io/media/audio-focus/transient_duck.html
// 3. Observe behavior.
// 4. Let YouTube video play and open Gmail in another page.
// 5. Send chat message from different device to this Gmail account to get notification (make sure notification is enabled from Gmail).
// 6. Observe video behavior.
func MTBF027NotificationsDuckExistingPlayback(ctx context.Context, s *testing.State) {
	contact, ok := s.Var("contact")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "contact"))
	}

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF027NotificationsDuckExistingPlayback",
		Description: "Short playbacks/notifications should duck the existing playback",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)

		if mtbferr := common.DriveDUT(ctx, s, "video.MTBF027ANotificationsDuckExistingPlayback"); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := compDev.SendHangoutsMessage(ctx, contact); mtbferr != nil {
			s.Fatal(mtbferr)
		}
		if mtbferr := common.DriveDUT(ctx, s, "video.MTBF027BNotificationsDuckExistingPlayback"); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		compDev := utils.NewDevice(client, common.CompanionID)
		common.DriveDUT(ctx, s, "video.MTBF027CNotificationsDuckExistingPlayback")
		cleanup027DUTAndCompanionPhone(ctx, compDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

func cleanup027DUTAndCompanionPhone(ctx context.Context, dut *utils.Device) {
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	dut.Client.Press(dut.DeviceID, ui.OprKeyEventHOME).Do(ctx)
}
