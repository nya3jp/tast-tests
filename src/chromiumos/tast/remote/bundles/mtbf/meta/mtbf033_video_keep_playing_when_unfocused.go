// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"cienet.com/cats/node/sdk"

	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/operations"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF033VideoKeepPlayingWhenUnfocused,
		Desc:     "ARC++ Youtube video should not pause while window focus shifted",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"coordinate.youtube", "cats.youtubeURL", "cats.youtubeTitle", "cats.requestURL"},
	})
}

func MTBF033VideoKeepPlayingWhenUnfocused(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF033VideoKeepPlayingWhenUnfocused",
		Description: "ARC++ Youtube video should not pause while window focus shifted",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := operations.OpenYoutubeAndPlay(ctx, s, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := common.DriveDUT(ctx, s, "mtbfutil.ClickSystemTray"); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := operations.VerifyYoutubePlaying(ctx, s, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		operations.CloseYoutube(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
