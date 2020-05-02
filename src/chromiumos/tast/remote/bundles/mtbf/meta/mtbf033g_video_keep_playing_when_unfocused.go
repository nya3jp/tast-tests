// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"cienet.com/cats/node/sdk"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/operations"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF033GVideoKeepPlayingWhenUnfocused,
		Desc:     "ARC++ Youtube video should not pause while window focus shifted",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"coordinate.youtube", "cats.youtubeURL", "cats.youtubeTitle", "cats.requestURL"},
	})
}

func MTBF033GVideoKeepPlayingWhenUnfocused(ctx context.Context, s *testing.State) {
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

		if err := operations.OpenYoutubeAndPlay(ctx, s, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := common.DriveDUT(ctx, s, "mtbfutil.ClickSystemTray"); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := operations.VerifyYoutubePlaying(ctx, s, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		operations.CloseYoutube(ctx, dutDev)
		return nil, nil
	})

	_ = report

	s.Log(report)
	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
