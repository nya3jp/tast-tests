// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/***
#4 Audio Test over USB via a Dock Station
Pre-Condition:
(Please note: Brand / Model number on test result)
1. External displays
2. Docking station
3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)
4. Headphone/Microphone/Speaker

Procedure:
1)  Boot-up and Sign-In to the device
2)  Connect ext-display to (Docking station)
3)  Connect (Powered Docking station) to Chromebook
4)  Connect (Headphone/Microphone/External Speaker) onto Dock station
5)  Open Chrome browser : www.youtube.com and play any video
6)  Open Camera or Audio Recorder app and records

Verification:
4)  By default under (Audio settings) menu "Output/Input - USB Audio/Mic should be [checked] associate with Docking
5)  Make sure video/audio playback without any issue
6)  Make sure video/audio playback without any issue

6)  Make sure recorded audio without any issue
https://www.youtube.com/watch?v=aqz-KE-bpKQ?autoplay=1
***/

// headphone pluging check command
//cras_test_client | grep *Headphone | grep yes
//(9e934263)      7:0        75 0.000000     yes              no  1619683090              HEADPHONE            2*Headphone

// check audio streaming
//cras_test_client --dump_audio_thread | grep 'Output dev'
//cras_test_client --dump_audio_thread | head -n 5 | grep 'Output dev'
//Output dev: acpd7219m98357: :1,2

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock4UsbAudio,
		Desc:         "Test headphone, microphone, Chrome OS speaker while docking/undocking",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         []string{"WWCBIP", "InternalDisplayCamera"},
		Pre:          chrome.LoggedIn(),
	})
}

func Dock4UsbAudio(ctx context.Context, s *testing.State) {

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

	s.Log("Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := dock4UsbAudioStep2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := dock4UsbAudioStep3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - connect (Headphone/Microphone/External Speaker) onto Dock station
	if err := dock4UsbAudioStep4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5 - play youtube
	youtube, err := dock4UsbAudioStep5(ctx, s, cr, tconn)
	if err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}
	defer youtube.CloseWindow(ctx, tconn)

	// step 6 - Open Camera or Audio Recorder app and records
	if err := dock4UsbAudioStep6(ctx, s); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

}

func dock4UsbAudioStep2(ctx context.Context, s *testing.State) error {

	s.Log("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.ExtDisp1Type, utils.ExtDisp1Index, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in ext-display to station")
	}

	return nil
}

func dock4UsbAudioStep3(ctx context.Context, s *testing.State) error {

	s.Log("Step 3 - Connect (Powered Docking station) to Chromebook")

	// plug in station
	if err := utils.ControlFixture(ctx, s, utils.StationType, utils.StationIndex, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "failed to plug in station")
	}

	return nil
}

func dock4UsbAudioStep4(ctx context.Context, s *testing.State) error {

	s.Log("Step 4 - Connect (Headphone/Microphone/External Speaker) onto Dock station")

	// there is no fixture control method nowadays

	return nil
}

func dock4UsbAudioStep5(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) (*ash.Window, error) {

	s.Log("Step 5 - Play youtube")

	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to play youtube")
	}

	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get youtube")
	}

	return youtube, nil
}

func dock4UsbAudioStep6(ctx context.Context, s *testing.State) error {

	s.Log("Step 6 - Open Camera or Audio Recorder app and records")

	if err := utils.CameraCheckPlayback(ctx, s, s.RequiredVar("InternalDisplayCamera")); err != nil {
		return errors.Wrap(err, "failed to check playback by fixture")
	}

	return nil
}
