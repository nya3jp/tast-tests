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
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/remote/cats/utils"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF030SwitchMusicPlayer,
		Desc:     "Android apps should gain focus (ARC++)",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

// drive030PlayGoogleMusicAgain plays google music again after open chrome browser and navigator to youtube.
func drive030PlayGoogleMusicAgain(ctx context.Context, dut *utils.Device) error {
	// # 3.Go to Google Play Music and click the play button
	testing.ContextLog(ctx, "Play google music again")
	if err := dut.Client.StartMainActivity(
		dut.DeviceID,
		"com.android.music.activitymanagement.TopLevelActivity",
		"com.google.android.music").Do(ctx); err != nil {
		return err
	}
	isPause, _ := dut.Client.UIAObjEventWait(dut.DeviceID,
		"id=com.google.android.music:id/pause::desc=Play", 5000, ui.ObjEventTypeAppear).Do(ctx)
	if isPause {
		// 3.Go to Google Play Music and click the play button to resume playing and Observe behavior
		dut.Client.UIAClick(dut.DeviceID).Selector("id=com.google.android.music:id/pause").Do(ctx)
		isStartRadio, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "text=START RADIO", 2000, ui.ObjEventTypeAppear).Do(ctx)
		if isStartRadio {
			dut.Client.UIAClick(dut.DeviceID).Selector("text=START RADIO").Do(ctx)
		}
	} else {
		return mtbferrors.New(mtbferrors.VLCAppNotPause, nil)
	}
	return nil
}

// drive030DUT runs a tast case of MTBF030PlaybackYoutube.
func drive030DUT(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "video.MTBF030PlaybackYoutube"); err != nil {
		return err
	}

	return nil
}

// cleanup030GoogleMusic cleans up google music app.
func cleanup030GoogleMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.EnterToAppAndVerify(ctx, "com.android.music.activitymanagement.TopLevelActivity", "com.google.android.music", "packagename=com.google.android.music")
	ok, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !ok {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.music:id/pause::desc=Play").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
}

// cleanup030Youtube cleans up youtube page on chrome.
func cleanup030Youtube(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "video.MTBF030CloseChrome"); err != nil {
		return err
	}

	return nil
}

func MTBF030SwitchMusicPlayer(ctx context.Context, s *testing.State) {
	caseDesc := sdk.CaseDescription{
		Name:        "meta.MTBF030SwitchMusicPlayer",
		Description: "Android apps should gain focus (ARC++)",
	}

	testRun := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		if mtbferr := dutDev.OpenGoogleMusicAndPlay(ctx); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := drive030DUT(ctx, s); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		if mtbferr := drive030PlayGoogleMusicAgain(ctx, dutDev); mtbferr != nil {
			s.Fatal(mtbferr)
		}

		return nil, nil
	}

	cleanUp := func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutID := ctx.Value(common.DutID).(string)
		dutDev := utils.NewDevice(client, dutID)

		cleanup030GoogleMusic(ctx, dutDev)
		cleanup030Youtube(ctx, s)
		return nil, nil
	}

	common.NodeDetachModeRunCase(ctx, s, caseDesc, testRun, cleanUp)
}
