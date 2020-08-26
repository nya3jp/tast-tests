// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"cienet.com/cats/node/sdk"
	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/operations"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF033VideoKeepPlayingWhenUnfocused,
		Desc:     "ARC++ Youtube video should not pause while window focus shifted",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"meta.coordYoutube", "meta.youtubeURL", "meta.youtubeTitle", "meta.requestURL"},
		ServiceDeps: []string{
			"tast.mtbf.svc.CommService",
			"tast.mtbf.ui.Shelf",
		},
		SoftwareDeps: []string{"chrome", "arc"},
	})
}

func MTBF033VideoKeepPlayingWhenUnfocused(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF033VideoKeepPlayingWhenUnfocused",
		Description: "ARC++ Youtube video should not pause while window focus shifted",
		Timeout:     5 * time.Minute,
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		c, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer c.Close(ctx)

		if mtbferr := operations.OpenYoutubeAndPlay(ctx, s, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		shelf := ui.NewShelfClient(c.Conn)
		if _, mtbferr := shelf.OpenSystemTray(ctx, &empty.Empty{}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		if mtbferr := operations.VerifyYoutubePlaying(ctx, s, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
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
