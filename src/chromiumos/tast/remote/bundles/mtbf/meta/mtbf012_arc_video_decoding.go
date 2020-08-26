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
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF012ARCVideoDecoding,
		Desc:         "ARC++ Test H264 H/W decoding",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"meta.requestURL"},
		Data:         []string{"test_video.mp4"},
		SoftwareDeps: []string{"chrome", "chrome_internal", "android_p"},
		ServiceDeps: []string{
			"tast.mtbf.svc.HistogramService",
			"tast.mtbf.svc.CommService",
		},
	})
}

func drive012DUT(ctx context.Context, dut *utils.Device) error {
	testing.ContextLog(ctx, "Open VLC player")
	dut.OpenVLCAndEnterToDownload(ctx)

	testing.ContextLog(ctx, "Select a file and use VLC player to play")
	dut.Client.UIAClick(dut.DeviceID).Selector("ID=org.videolan.vlc:id/title::text=videos").Do(ctx)
	video, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=test_video.mp4", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !video {
		return mtbferrors.New(mtbferrors.FoundVideoFile, nil)
	}

	dut.Client.UIAClick(dut.DeviceID).Selector("text=test_video.mp4").Snapshot(false).Do(ctx, service.Sleep(0))
	dut.Client.UIASwipe(dut.DeviceID, 1275, 512, 548, 512, 10).Do(ctx)
	return nil
}

func cleanup012DUT(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.EnterToAppAndVerify(ctx, ".StartActivity", "org.videolan.vlc", "packagename=org.videolan.vlc")
	dut.PressCancelButton(ctx, 5)
	dut.ClickSelector(ctx, "text=OK")
	dut.PressCancelButton(ctx, 1)
	dut.ClickSelector(ctx, "text=OK")
}

func MTBF012ARCVideoDecoding(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF012ARCVideoDecoding",
		Description: "ARC++ Test H264 H/W decoding",
		Timeout:     time.Minute * 5,
	}

	common.VideoFilesPrepare(ctx, s, []string{"test_video.mp4"})

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		hsc := svc.NewHistogramServiceClient(cl.Conn)

		const histogramName = "Media.GpuArcVideoDecodeAccelerator.InitializeResult"

		rsp, mtbferr := hsc.GetFirstBucket(ctx, &svc.GetFirstBucketRequest{Name: histogramName})
		if mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		beforePlaying := rsp.Count

		if mtbferr := drive012DUT(ctx, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		rsp, mtbferr = hsc.GetFirstBucket(ctx, &svc.GetFirstBucketRequest{Name: histogramName})
		if mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		if rsp.Count <= beforePlaying {
			common.Fatal(ctx, s, mtbferrors.New(mtbferrors.VideoHistCount, nil, rsp.Count))
		}

		s.Logf("The first bucket count of [%v] = %v", histogramName, rsp.Count)

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		testing.ContextLog(ctx, "Start case cleanup")
		cleanup012DUT(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
