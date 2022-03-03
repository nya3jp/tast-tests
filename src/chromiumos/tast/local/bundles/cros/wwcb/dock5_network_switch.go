// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/***
#8 Wired/WiFi network switching over Dock
Pre-Condition:
(Please note: Brand / Model number on test result)
1. External displays
2. Docking station / Hub
3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)
4. Wired and WiFi connection (Router / Wireless Hub)

Procedure:
1) Boot-up and Sign-In to the device
2) Connect ext-display to (Docking station)
3) Connect (Docking station) to Chromebook
4) Connect wired Ethernet cable onto (Dock station or Hub)
5) Open Chrome Browser: www.youtube.com and play any video
6) Disconnect Ethernet cable, and connect to WiFi
7) Repeat step: #5

Verification:
4)  Make sure (Quick Settings Menu) show "Ethernet" connection
HideAllNotifications
5)  Make sure video/audio playback without any issue
6)  Make sure (Quick Setting Menu) show "WiFi" connection
7)  Make sure video/audio playback without any issue
*/

// headphone pluging check command
//cras_test_client | grep *Headphone | grep yes
//(9e934263)      7:0        75 0.000000     yes              no  1619683090              HEADPHONE            2*Headphone

// check eth0
// Ethernet : ifconfig eth0 | grep inet
// wifi : ifconfig wlan0 | grep inet
//Output dev: acpd7219m98357: :1,2
// enable/disable wifi : ifconfig wlan0 up/down

/***
2021/05/06 23:43:30 --------------------------------------------------------------------------------
2021/05/06 23:43:30 wwcb.Dock5NetworkSwitch.ethernet  [ FAIL ] Lost SSH connection: target did not come back: context deadline exceeded; last error follows: dial tcp 192.168.0.102:22: i/o timeout
2021/05/06 23:43:30                                                 Test did not finish
2021/05/06 23:43:30 --------------------------------------------------------------------------------
*/

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock5NetworkSwitch,
		Desc:         "Test wired/WiFi network switching when connecting/disconnecting over a Dock",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"WWCBIP", "InternalDisplayCamera"},
		Pre:          chrome.LoggedIn(), // 1) Boot-up and Sign-In to the device
	})
}

func Dock5NetworkSwitch(ctx context.Context, s *testing.State) {
	// set up
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	s.Log("Step 1 - Boot-up and Sign-In to the device ")

	// step 2 - connect ext-display to station
	if err := dock5NetworkSwitchStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := dock5NetworkSwitchStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - connect ethernet to station
	if err := dock5NetworkSwitchStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5 - play youtube then check playback
	if err := dock5NetworkSwitchStep5(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step5: ", err)
	}

	// step 6 - disconnect ethernet from station
	if err := dock5NetworkSwitchStep6(ctx, s); err != nil {
		s.Fatal("Failed to execute step6: ", err)
	}

	// step 7 - play youtube then check playback
	if err := dock5NetworkSwitchStep7(ctx, s, cr, tconn); err != nil {
		s.Fatal("Failed to execute step7: ", err)
	}

}

func dock5NetworkSwitchStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to connect ext-display to docking station")
	}

	return nil
}

func dock5NetworkSwitchStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect docking station to chromebook ")

	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in docking station to chromebook")
	}

	return nil
}

func dock5NetworkSwitchStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect wire ethernet to docking station ")

	// plug in ethernet
	if err := utils.ControlFixture(ctx, s, utils.EthernetType, utils.EthernetIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in ethernet")
	}

	// check ethernet status in 30s
	if err := testing.Poll(ctx, func(c context.Context) error {

		testing.Sleep(ctx, 1*time.Second)

		if err := utils.VerifyEthernetStatus(ctx, utils.IsConnect); err != nil {
			return err
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return err
	}

	return nil
}

func dock5NetworkSwitchStep5(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 5 - Open brower and play youtube")

	// play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	// 5)  Make sure video/audio playback without any issue
	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback on internal display")
	}

	// get youtube window
	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	}

	// close youtube in the end
	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close youtube")
	}

	return nil

}

func dock5NetworkSwitchStep6(ctx context.Context, s *testing.State) error {

	s.Log("Step 6 - Disconnect ethernet cable, and connect to WiFi")

	// disconnect ethernet cable
	if err := utils.ControlFixture(ctx, s, utils.EthernetType, utils.EthernetIndex, utils.ActionUnplug, false); err != nil {
		return errors.Wrap(err, "failed to unplug ethernet")
	}

	// check network interface is disabled or not in 30s
	if err := testing.Poll(ctx, func(c context.Context) error {

		testing.Sleep(ctx, 1*time.Second)

		if err := utils.VerifyEthernetStatus(ctx, utils.IsDisconnect); err != nil {
			return err
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {

		return err
	}

	return nil
}

func dock5NetworkSwitchStep7(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) error {

	s.Log("Step 7 - Play youtube")

	// play youtube
	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to play youtube")
	}

	// make sure video playback without any issue
	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback on internal display")
	}

	// get youtube window
	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get youtube window")
	}

	// close youtube window
	if err := youtube.CloseWindow(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to close youtube")
	}

	return nil
}
