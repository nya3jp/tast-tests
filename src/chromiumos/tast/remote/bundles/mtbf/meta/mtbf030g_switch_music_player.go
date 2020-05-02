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
		Func:     MTBF030GSwitchMusicPlayer,
		Desc:     "Android apps should gain focus (ARC++)",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
		Vars:     []string{"cats.requestURL"},
	})
}

// drive030GPlayGoogleMusicAgain plays google music again after open chrome browser and navigator to youtube.
func drive030GPlayGoogleMusicAgain(ctx context.Context, dut *utils.Device) error {
	// # 3.Go to Google Play Music and click the play button
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

// drive030GDUT runs a tast case of MTBF030PlaybackYoutube.
func drive030GDUT(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "video.MTBF030PlaybackYoutube"); err != nil {
		return err
	}

	return nil
}

// cleanup030GGoogleMusic cleans up google music app.
func cleanup030GGoogleMusic(ctx context.Context, dut *utils.Device) {
	dut.Client.Comments("Recover env").Do(ctx)
	dut.EnterToAppAndVerify(ctx, "com.android.music.activitymanagement.TopLevelActivity", "com.google.android.music", "packagename=com.google.android.music")
	ok, _ := dut.Client.UIAObjEventWait(dut.DeviceID, "ID=com.google.android.music:id/pause::desc=Play", 6000, ui.ObjEventTypeAppear).Do(ctx)
	if !ok {
		dut.Client.UIAClick(dut.DeviceID).Selector("ID=com.google.android.music:id/pause::desc=Play").Do(ctx)
	}
	dut.PressCancelButton(ctx, 3)
}

// cleanup030GYoutube cleans up youtube page on chrome.
func cleanup030GYoutube(ctx context.Context, s *testing.State) error {
	flags := common.GetFlags(s)
	if err := tastrun.RunTestWithFlags(ctx, s, flags, "video.MTBF030CloseChrome"); err != nil {
		return err
	}

	return nil
}

func MTBF030GSwitchMusicPlayer(ctx context.Context, s *testing.State) {
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

		if err := dutDev.OpenGoogleMusicAndPlay(ctx); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive030GDUT(ctx, s); err != nil {
			utils.FailCase(ctx, client, err)
		}

		if err := drive030GPlayGoogleMusicAgain(ctx, dutDev); err != nil {
			utils.FailCase(ctx, client, err)
		}

		return nil, nil
	}, func(ctx context.Context, client sdk.DelegateClient) (interface{}, error) {
		dutDev := utils.NewDevice(client, dutID)

		cleanup030GGoogleMusic(ctx, dutDev)
		cleanup030GYoutube(ctx, s)
		return nil, nil
	})

	_ = report

	if err != nil {
		s.Error("CATS test failed: ", err)
	}
}
