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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock4UsbAudio,
		Desc:         "Test headphone, microphone, Chrome OS speaker while docking/undocking",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
		Pre:          chrome.LoggedIn(),
		Data:         []string{"test_audio.wav"},
	})
}

func Dock4UsbAudio(ctx context.Context, s *testing.State) {
	dockingID := s.RequiredVar("DockingID")
	extDispID1 := s.RequiredVar("1stExtDispID")

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create a keyboard: ", err)
	}
	defer kb.Close()

	// copy file to downloads
	testAudio := "test_audio.wav"
	testAudioLocation := filepath.Join(filesapp.DownloadPath, testAudio)
	if err := fsutil.CopyFile(s.DataPath(testAudio), testAudioLocation); err != nil {
		s.Fatalf("Failed to copy the golden audio file to %s: %s", testAudioLocation, err)
	}
	defer os.Remove(testAudioLocation)

	if err := utils.InitFixtures(ctx); err != nil {
		s.Fatal("Failed to initialize fixtures: ", err)
	}

	testing.ContextLog(ctx, "Step 1 - Boot-up and Sign-In to the device")

	// step 2 - connect ext-display to station
	if err := dock4UsbAudioStep2(ctx, extDispID1); err != nil {
		s.Fatal("Failed to execute step2: ", err)
	}
	// step 3 - connect station to chromebook
	if err := dock4UsbAudioStep3(ctx, tconn, dockingID); err != nil {
		s.Fatal("Failed to execute step3: ", err)
	}
	// step 4 - connect (Headphone/Microphone/External Speaker) onto Dock station
	if err := dock4UsbAudioStep4(ctx); err != nil {
		s.Fatal("Failed to execute step4: ", err)
	}
	// step 5 - play audio file and record
	// step 6 - audio comparison
	if err := dock4UsbAudioStep5and6(ctx, cr, tconn, kb, testAudio); err != nil {
		s.Fatal("Failed to execute step 5 and 6: ", err)
	}
}

func dock4UsbAudioStep2(ctx context.Context, extDispID1 string) error {
	testing.ContextLog(ctx, "Step 2 - Connect ext-display to docking station")
	if err := utils.SwitchFixture(ctx, extDispID1, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect external display")
	}
	return nil
}

func dock4UsbAudioStep3(ctx context.Context, tconn *chrome.TestConn, dockingID string) error {
	testing.ContextLog(ctx, "Step 3 - Connect docking station to Chromebook")
	if err := utils.SwitchFixture(ctx, dockingID, "on", "0"); err != nil {
		return errors.Wrap(err, "failed to connect docking station")
	}
	if err := utils.VerifyDisplayProperly(ctx, tconn, 2); err != nil {
		return errors.Wrap(err, "failed to verify display properly")
	}
	return nil
}

func dock4UsbAudioStep4(ctx context.Context) error {
	testing.ContextLog(ctx, "Step 4 - Connect (Headphone/Microphone/External Speaker) onto Dock station")
	// connect devices in pre-setting state

	// When docking station is connected, input & output channel would change to usb
	return nil
}

func dock4UsbAudioStep5and6(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, testAudio string) error {
	testing.ContextLog(ctx, "Step 5 - Play audio file and record")

	// open file in downloads
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch the files app")
	}
	defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "failed to open Downloads folder in files app")
	}
	if err := files.OpenFile(testAudio)(ctx); err != nil {
		return errors.Wrap(err, "failed to open the audio file")
	}
	// record audio for 15s
	const (
		audioRate      = 48000
		audioChannel   = 2
		recWavFileName = "15SEC_REC.wav"
	)
	recWavFile := filepath.Join(filesapp.DownloadPath, recWavFileName)
	cmd := fmt.Sprintf("rec -r %d -c %d %s trim 0 15", audioRate, audioChannel, recWavFile)
	output := testexec.CommandContext(ctx, "sh", "-c", cmd)
	if err := output.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to execute %q", cmd)
	}

	// Deleting the files and closing the audio player.
	defer func() {
		os.Remove(recWavFile)
		kb.Accel(ctx, "Ctrl+W")
	}()

	// upload file to wwcb server then do audio comparison
	testing.ContextLog(ctx, "Step 6 - Audio comparison")
	uploadPath, err := utils.UploadFile(ctx, recWavFile)
	if err != nil {
		return errors.Wrap(err, "failed to upload file to wwcb server")
	}
	if err := utils.DetectAudio(ctx, uploadPath); err != nil {
		return errors.Wrap(err, "failed to detect audio")
	}
	return nil
}
