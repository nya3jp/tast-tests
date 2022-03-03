// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/***
#15 USB Charging via a powered Dock
Pre-Condition:
(Please note: Brand / Model number on test result)
1. External displays
2. Docking station
3. Connection Type (HDMI/DP/VGA/DVI/USB-C on test result)

Procedure:
1)  Boot-up and Sign-In to the device
2)  Connect ext-display to (Powered Docking station)
3)  Connect (Powered Docking station) to Chromebook


Verification:
- Chrome Book /Chrome Box "Battery" icon should show (Lighting Bolt charging) indicator
***/

// Package wwcb contains local Tast tests that work with chromebook
package wwcb

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/wwcb/utils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Dock3UsbCharging2,
		Desc:         "Test power charging via a powered Dock over USB-C",
		Contacts:     []string{"flin@google.com", "newmanliu19020@allion.corp-partner.google.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Vars:         []string{"WWCBIP", "DockingID", "1stExtDispID"},
		Pre:          chrome.LoggedIn(),
		Data:         []string{"test_audio.wav"},
	})
}

func Dock3UsbCharging2(ctx context.Context, s *testing.State) {

	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	testAudio := "test_audio.wav"

	// copy file to downloads
	testAudioLocation := filepath.Join(filesapp.DownloadPath, testAudio)
	if err := fsutil.CopyFile(s.DataPath(testAudio), testAudioLocation); err != nil {
		s.Fatalf("Failed to copy the golden audio file to %s: %s", testAudioLocation, err)
	}
	// defer os.Remove(testAudioLocation)

	// open file in downloads
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	// defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(testAudio)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", testAudio, err)
	}

	const (
		audioRate    = 48000
		audioChannel = 2
		duration     = 30
	)
	recWavFileName := "20SEC_REC.wav"
	recWavFile := filepath.Join(filesapp.DownloadPath, recWavFileName)
	cmd := fmt.Sprintf("rec -r %d -c %d %s trim 0 20", audioRate, audioChannel, recWavFile)
	output := testexec.CommandContext(ctx, "sh", "-c", cmd)
	if err := output.Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Failed to execute %q: %v", cmd, err)
	}

	uploadPath, err := utils.UploadFile(ctx, recWavFile)
	if err != nil {
		s.Fatal("Failed to upload file: ", err)
	}
	if err := utils.DetectAudio(ctx, uploadPath); err != nil {
		s.Fatal("Failed to detect audio: ", err)
	}
}
