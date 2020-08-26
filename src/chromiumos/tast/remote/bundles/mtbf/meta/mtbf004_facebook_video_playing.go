// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"
	"time"

	"cienet.com/cats/node/sdk"
	"cienet.com/cats/node/sdk/ui"
	"cienet.com/cats/node/service"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/remote/bundles/mtbf/meta/cats/utils"
	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/mtbf/multimedia"
	"chromiumos/tast/services/mtbf/video"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MTBF004FacebookVideoPlaying,
		Desc:         "ARC++ Test Facebook video apps",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		Vars:         []string{"meta.coordFacebook", "meta.requestURL", "meta.intentURL"},
		SoftwareDeps: []string{"chrome", "arc"},
		ServiceDeps: []string{
			"tast.mtbf.video.FacebookService",
			"tast.mtbf.multimedia.VolumeService",
			"tast.mtbf.svc.CommService",
		},
	})
}

/*
Precondition:
Install facebook app from the Play Store.

Procedure:
1. Open above mentioned apps one after other and run below steps.
2. Check video playback.
3. Seek video to different positions.
4. Change resolution settings if supported
5. Play in full screen if supported
6. Observe audio controls behavior.

Verification:
2.1 Video can be played.
3.1 Video should play from the seek position.
4.1 Video should play with new resolution.
5.1 Video should play with full screen.
6.1 Audio levels should be effected only with ChromeOS audio controls. (ie. Device volume level doesn't change if changing volues inside Android APP)
*/

func MTBF004FacebookVideoPlaying(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF004FacebookVideoPlaying",
		Description: "ARC++ Test Facebook video apps",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "mtbf")
		if err != nil {
			s.Fatal(mtbferrors.New(mtbferrors.GRPCDialFail, err))
		}
		defer cl.Close(ctx)

		url := s.RequiredVar("meta.intentURL")
		fsc := video.NewFacebookServiceClient(cl.Conn)
		if _, mtbferr := fsc.OpenFacebook(ctx, &video.OpenFacebookRequest{IntentURL: url}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		if mtbferr := verifyFacebookVideoPlaying(ctx, s, dutDev); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		vsc := multimedia.NewVolumeServiceClient(cl.Conn)
		if _, mtbferr := vsc.Set(ctx, &multimedia.VolumeRequest{Value: 10}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}

		if _, mtbferr := vsc.Set(ctx, &multimedia.VolumeRequest{Value: 100}); mtbferr != nil {
			common.Fatal(ctx, s, mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dut := utils.NewDevice(client, dutID)
		client.Comments("Close Facebook").Do(ctx)
		dut.PressCancelButton(ctx, 8)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

// verifyFacebookVideoPlaying verify facebook video is playing
func verifyFacebookVideoPlaying(ctx context.Context, s *testing.State, dut *utils.Device) error {
	const (
		facebookID      = "ID=com.facebook.katana:id/(name removed)::"
		pauseBtnSel     = facebookID + "desc=Pause current video"
		videoQualitySel = facebookID + "desc=Video Quality"
		imageViewSel    = facebookID + "class=android.widget.ImageView"
		mostRecentSel   = "text=Most Recent"
		fullscreenSel   = "desc=Fullscreen"
		viewGroupSel    = "class=android.view.ViewGroup::index=1"
		recycleViewSel  = "class=androidx.recyclerview.widget.RecyclerView"
		btnClassSel     = "class=android.widget.Button"
		seekBarClass    = "android.widget.SeekBar"
	)

	s.Log("Wait for 'Most Recent' to appear")
	mostRecentAppear, _ := dut.Client.UIAObjEventWait(dut.DeviceID, mostRecentSel, 15000, ui.ObjEventTypeAppear).Do(ctx)
	if !mostRecentAppear {
		return mtbferrors.New(mtbferrors.FacebookVideoNotPlay, nil)
	}
	title, _ := dut.Client.UIASearchElement(dut.DeviceID, mostRecentSel).Do(ctx)
	if !title.Found {
		return mtbferrors.New(mtbferrors.CannotPlayFacebookVideo, nil)
	}
	s.Log("Click the video below 'Most Recent'")
	coord := fmt.Sprintf("%d,%d", title.CenterX, title.CenterY+60)
	dut.ClickCoordinate(ctx, coord)

	s.Log("Search the full screen button")
	fsBtn, _ := dut.Client.UIASearchElement(dut.DeviceID, fullscreenSel).ParentSelector(viewGroupSel).Do(ctx, service.Suppress())
	if fsBtn.Found {
		coord = fmt.Sprintf("%d,%d", fsBtn.CenterX, fsBtn.CenterY+200)
		s.Logf("Found video coordinate = %s", coord)
	} else {
		coord = s.RequiredVar("meta.coordFacebook")
		s.Logf("Use default coordinate = %s", coord)
	}

	dut.Client.Delay(3000).Do(ctx)
	s.Log("Click video and verify playing")
	dut.ClickCoordinate(ctx, coord)
	play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, pauseBtnSel, 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !play {
		s.Log("Retry verify playing")
		dut.ClickCoordinate(ctx, coord)
		play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, pauseBtnSel, 5000, ui.ObjEventTypeAppear).Do(ctx)
		if !play {
			return mtbferrors.New(mtbferrors.FacebookVideoNotPlay, nil)
		}
	}
	dut.Client.Delay(3000).Do(ctx)

	s.Log("Verify seek bar")
	dut.ClickCoordinate(ctx, coord)
	dut.Client.ScrollListItem(dut.DeviceID, ui.ScrollDirectionsLEFT, ui.NewUiNodePropDesc().Class(seekBarClass)).Do(ctx, service.Sleep(time.Second*5))

	s.Log("Verify video quality")
	dut.ClickCoordinate(ctx, coord)
	dut.Client.UIAClick(dut.DeviceID).Selector(videoQualitySel).Do(ctx)

	res, _ := dut.Client.UIASearchSequentialElement(dut.DeviceID, recycleViewSel, 2).TargetSelector(btnClassSel).Do(ctx)
	if res.Found {
		targetResSel := "text=" + res.ContentDescription
		dut.Client.UIAClick(dut.DeviceID).Selector(targetResSel).Do(ctx)
		chg, _ := dut.Client.UIAVerifyListItem(dut.DeviceID, imageViewSel).
			ReferenceSelector(facebookID + targetResSel).
			ScrollSelector(recycleViewSel).Do(ctx)
		if !chg {
			return mtbferrors.New(mtbferrors.VideoResolutionNotExpected, nil)
		}
		dut.PressCancelButton(ctx, 1)
	}

	s.Log("Search pauseBtn.beforeFull")
	dut.ClickCoordinate(ctx, coord)
	beforeFull, _ := dut.Client.UIASearchElement(dut.DeviceID, pauseBtnSel).Do(ctx)
	if !beforeFull.Found {
		dut.Client.Delay(5000).Do(ctx)
		s.Log("Retry searching pauseBtn.beforeFull")
		dut.ClickCoordinate(ctx, coord)
		beforeFull, _ = dut.Client.UIASearchElement(dut.DeviceID, pauseBtnSel).Do(ctx)
	}

	dut.Client.Delay(3000).Do(ctx)
	s.Log("Switch to full screen")
	dut.ClickCoordinate(ctx, coord)
	dut.Client.UIAClick(dut.DeviceID).Selector(fullscreenSel).Do(ctx, service.Sleep(time.Second*3))

	s.Log("Search pauseBtn.afterFull")
	dut.ClickCoordinate(ctx, coord)
	afterFull, _ := dut.Client.UIASearchElement(dut.DeviceID, pauseBtnSel).Do(ctx)
	if !afterFull.Found {
		dut.Client.Delay(5000).Do(ctx)
		s.Log("Retry searching pauseBtn.afterFull")
		dut.ClickCoordinate(ctx, coord)
		afterFull, _ = dut.Client.UIASearchElement(dut.DeviceID, pauseBtnSel).Do(ctx)
	}

	if beforeFull.Found && afterFull.Found {
		if afterFull.CenterY > beforeFull.CenterY {
			dut.Client.Comments("Screen becomes full screen.").Do(ctx)
		}
	} else {
		return mtbferrors.New(mtbferrors.VerifyResolution, nil)
	}

	return nil
}
