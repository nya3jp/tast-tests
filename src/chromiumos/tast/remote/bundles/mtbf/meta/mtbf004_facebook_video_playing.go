// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"fmt"

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
		Func:     MTBF004FacebookVideoPlaying,
		Desc:     "ARC++ Test Facebook video apps",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"coordinate.facebook.1366", "coordinate.facebook", "coordinate.facebook.saved_2400", "cats.requestURL"},
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

		if mtbferr := verifyFacebookVideoPlaying(ctx, s, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := common.DriveDUT(ctx, s, "video.MTBF004AdjustVolume.10"); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := common.DriveDUT(ctx, s, "video.MTBF004AdjustVolume.100"); mtbferr != nil {
			s.Fatal(mtbferr)
		}
		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)
		closeFacebook(ctx, dutDev)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}

// verifyFacebookVideoPlaying verify facebook video is playing
func verifyFacebookVideoPlaying(ctx context.Context, s *testing.State, dut *utils.Device) error {
	coord := common.GetVar(ctx, s, "coordinate.facebook")

	if err := dut.EnterToAppAndVerify(ctx,
		".LoginActivity",
		"com.facebook.katana",
		"ID=com.facebook.katana:id/(name removed)::index=3",
	); err != nil {
		return err
	}
	if err := dut.Client.UIAClick(dut.DeviceID).Selector(
		"ID=com.facebook.katana:id/(name removed)::index=5",
	).Do(ctx); err != nil {
		return err
	}
	s.Log("Click 'Saved'")
	hasSaved, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "desc=Saved", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if hasSaved {
		dut.Client.UIAClick(dut.DeviceID).Selector("desc=Saved").Do(ctx)
	} else {
		coord2400 := common.GetVar(ctx, s, "coordinate.facebook.saved_2400")
		dut.Client.UIAClick(dut.DeviceID).Coordinate(coord2400).Snapshot(false).Do(ctx, service.Sleep(0))
	}
	title, _ := dut.Client.UIASearchElement(dut.DeviceID, "text=Most Recent").Do(ctx)
	s.Log("Found 'Most Recent' and click video")
	if title.Found {
		dut.Client.UIAClick(dut.DeviceID).Coordinate(
			fmt.Sprintf("%d,%d", title.CenterX, title.CenterY+60),
		).Do(ctx)
	} else {
		return mtbferrors.New(mtbferrors.CannotPlayFacebookVideo, nil)
	}

	dut.Client.Delay(3000).Do(ctx)
	s.Log("Click video and verify playing")
	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
	play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.facebook.katana:id/(name removed)::desc=Pause current video", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if !play {
		dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
		play, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.facebook.katana:id/(name removed)::desc=Pause current video", 5000, ui.ObjEventTypeAppear).Do(ctx)
		if !play {
			return mtbferrors.New(mtbferrors.FacebookVideoNotPlay, nil)
		}
	}

	dut.Client.Delay(3000).Do(ctx)

	s.Log("Verify video quality")
	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
	dut.Client.ScrollListItem(dut.DeviceID, ui.ScrollDirectionsLEFT, ui.NewUiNodePropDesc().Class("android.widget.SeekBar")).Do(ctx, service.Sleep(4))
	dut.Client.Delay(3000).Do(ctx)

	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
	dut.Client.UIAClick(dut.DeviceID).Selector(
		"ID=com.facebook.katana:id/(name removed)::desc=Video Quality",
	).Do(ctx, service.Sleep(0))

	res, _ := dut.Client.UIASearchSequentialElement(dut.DeviceID, "class=androidx.recyclerview.widget.RecyclerView", 2).TargetSelector("class=android.widget.Button").Do(ctx)
	if res.Found {
		targetRes := res.ContentDescription
		dut.Client.UIAClick(dut.DeviceID).Selector("text="+targetRes).Do(ctx, service.Sleep(0))
		chg, _ := dut.Client.UIAVerifyListItem(dut.DeviceID,
			"ID=com.facebook.katana:id/(name removed)::class=android.widget.ImageView").ReferenceSelector(
			"ID=com.facebook.katana:id/(name removed)::text=" + targetRes).ScrollSelector(
			"class=androidx.recyclerview.widget.RecyclerView").Do(ctx)
		if !chg {
			return mtbferrors.New(mtbferrors.VideoResolutionNotExpected, nil)
		}
		dut.PressCancelButton(ctx, 1)
	}

	s.Log("Verify fullscreen")
	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
	dut.Client.ScrollListItem(dut.DeviceID, ui.ScrollDirectionsLEFT, ui.NewUiNodePropDesc().Class("android.widget.SeekBar")).Do(ctx, service.Sleep(4))
	dut.Client.Delay(3000).Do(ctx)

	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
	beforeFull, _ := dut.Client.UIASearchElement(dut.DeviceID,
		"ID=com.facebook.katana:id/(name removed)::desc=Pause current video").Do(ctx)
	dut.Client.UIAClick(dut.DeviceID).Selector("desc=Fullscreen").Do(ctx, service.Sleep(3000))
	dut.Client.Delay(3000).Do(ctx)
	dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
	afterFull, _ := dut.Client.UIASearchElement(dut.DeviceID,
		"ID=com.facebook.katana:id/(name removed)::desc=Pause current video").Do(ctx)
	if !afterFull.Found {
		s.Log("Retry get element")
		dut.Client.Click(dut.DeviceID).NodeProp(ui.NewUiNodePropDesc().Coordinate(coord)).Do(ctx, service.Sleep(0))
		afterFull, _ := dut.Client.UIASearchElement(dut.DeviceID,
			"ID=com.facebook.katana:id/(name removed)::desc=Pause current video").Do(ctx)
		if !afterFull.Found {
			return mtbferrors.New(mtbferrors.VerifyResolution, nil)
		}
	}

	if beforeFull.Found && afterFull.Found {
		if afterFull.CenterX > beforeFull.CenterX {
			dut.Client.Comments("Screen becomes full screen.").Do(ctx)
		}
	} else {
		return mtbferrors.New(mtbferrors.VerifyResolution, nil)
	}

	return nil
}

// closeFacebook close app
func closeFacebook(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.PressCancelButton(ctx, 8)
}
