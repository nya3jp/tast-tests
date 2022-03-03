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

package crostini

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/crostini/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
	"context"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock4UsbAudio,
		Desc:         "Test headphone, microphone, Chrome OS speaker while docking/undocking",
		Contacts:     []string{"allion-sw@allion.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      10 * time.Minute,
		Vars:         utils.GetInputVars(),
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

	s.Logf("Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := Dock4UsbAudio_Step2(ctx, s); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}

	// step 3 - connect station to chromebook
	if err := Dock4UsbAudio_Step3(ctx, s); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}

	// step 4 - detect devices
	if err := Dock4UsbAudio_Step4(ctx, s); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}

	// step 5 - open youtube
	youtube, err := Dock4UsbAudio_Step5(ctx, s, cr, tconn)
	if err != nil {
		s.Fatal("Failed to execute step 5: ", err)
	}
	defer youtube.CloseWindow(ctx, tconn)

	// step 6 - Open Camera or Audio Recorder app and records
	if err := Dock4UsbAudio_Step6(ctx, s); err != nil {
		s.Fatal("Failed to execute step 6: ", err)
	}

}

// 2)  Connect ext-display to (Docking station)
func Dock4UsbAudio_Step2(ctx context.Context, s *testing.State) error {

	s.Logf("Step 2 - Connect ext-display to docking station")

	if err := utils.ControlFixture(ctx, s, utils.FixtureExtDisp1, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in ext-display to station: ")
	}

	return nil
}

// 3)  Connect (Powered Docking station) to Chromebook
func Dock4UsbAudio_Step3(ctx context.Context, s *testing.State) error {

	s.Logf("Step 3 - Connect (Powered Docking station) to Chromebook")

	// plug in station
	if err := utils.ControlFixture(ctx, s, utils.FixtureStation, utils.ActionPlugin, false); err != nil {
		return errors.Wrap(err, "Failed to plug in station: ")
	}

	return nil
}

// 4)  Connect (Headphone/Microphone/External Speaker) onto Dock station
func Dock4UsbAudio_Step4(ctx context.Context, s *testing.State) error {

	s.Logf("Step 4 - Connect (Headphone/Microphone/External Speaker) onto Dock station")

	// there is no fixture control method nowadays

	return nil
}

// 5)  Open Chrome browser : www.youtube.com and play any video
func Dock4UsbAudio_Step5(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) (*ash.Window, error) {

	s.Logf("Step 5 - Play youtube")

	if err := utils.PlayYouTube(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "Failed to play youtube: ")
	}

	youtube, err := utils.GetYoutubeWindow(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get youtube: ")
	}

	return youtube, nil
}

// 6)  Open Camera or Audio Recorder app and records
func Dock4UsbAudio_Step6(ctx context.Context, s *testing.State) error {

	s.Logf("Step 6 - Open Camera or Audio Recorder app and records")

	if err := utils.CheckPlaybackByFixture(ctx, s, utils.InternalDisplay); err != nil {
		return errors.Wrap(err, "Failed to check playback by fixture: ")
	}

	return nil
}
