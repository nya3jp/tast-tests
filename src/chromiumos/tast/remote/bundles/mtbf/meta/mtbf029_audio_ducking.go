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
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF029AudioDucking,
		Desc:     "Android notifications should duck the existing playback (ARC++)",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{"format_m4a.m4a"},
		Vars: []string{
			"meta.requestURL",
			"contact",
		},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

func drive029DUTAndCompanionPhone(ctx context.Context, dutDev, compDev *utils.Device, contact string) error {
	testing.ContextLog(ctx, "Open google music and play")
	dutDev.OpenGoogleMusicAndPlay(ctx)

	compDev.SendHangoutsMessage(ctx, contact)
	return nil
}

func cleanup029DUTAndCompanionPhone(ctx context.Context, dutDev, compDev *utils.Device) {
	compDev.Client.Press(compDev.DeviceID, ui.OprKeyEventCANCEL).Times(3).Do(ctx)
	compDev.Client.Press(compDev.DeviceID, ui.OprKeyEventHOME).Do(ctx)

	pause, _ := dutDev.Client.UIAObjEventWait(dutDev.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !pause {
		dutDev.Client.UIAClick(dutDev.DeviceID).Selector("ID=com.google.android.music:id/pause").Do(ctx, service.Sleep(time.Second*2))
	}

	dutDev.PressCancelButton(ctx, 1)
}

func MTBF029AudioDucking(ctx context.Context, s *testing.State) {
	contact, ok := s.Var("contact")
	if !ok {
		s.Fatal(mtbferrors.New(mtbferrors.OSVarRead, nil, "contact"))
	}

	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF029AudioDucking",
		Description: "Android notifications should duck the existing playback (ARC++)",
	}

	common.AudioFilesPrepare(ctx, s, []string{"format_m4a.m4a"})

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)
		compDev := utils.NewDevice(client, common.CompanionID)

		if mtbferr := drive029DUTAndCompanionPhone(ctx, dutDev, compDev, contact); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)
		compDev := utils.NewDevice(client, common.CompanionID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup029DUTAndCompanionPhone(ctx, dutDev, compDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
