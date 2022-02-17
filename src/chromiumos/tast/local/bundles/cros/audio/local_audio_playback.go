// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalAudioPlayback,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Play local audio file through default app and check if the audio is routing through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Params: []testing.Param{{
			Name:      "internal_speaker",
			ExtraAttr: []string{"group:mainline", "informational"},
			Val:       "Speaker (internal)",
		}, {
			Name: "headphone",
			Val:  "Headphone",
		}, {
			Name: "usb_speaker",
			Val:  "USB",
		}},
	})
}

// LocalAudioPlayback generates audio file and plays it through default audio player.
func LocalAudioPlayback(ctx context.Context, s *testing.State) {
	expectedOutputDevice := s.Param().(string)
	cr := s.PreValue().(*chrome.Chrome)

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(ctx)

	// Generate sine raw input file that lasts 30 seconds.
	rawFileName := "30SEC.raw"
	rawFilePath := filepath.Join(filesapp.DownloadPath, rawFileName)
	rawFile := audio.TestRawData{
		Path:          rawFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      30,
	}
	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		s.Fatal("Failed to generate audio test data: ", err)
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "30SEC.wav"
	wavFile := filepath.Join(filesapp.DownloadPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", wavFileName, err)
	}
	// Closing the audio player.
	defer func() {
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}()

	// Sample time for the audio to play for 5 seconds.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Error while waiting during sample time: ", err)
	}

	if err := quicksettings.SelectAudioOption(ctx, tconn, expectedOutputDevice); err != nil {
		s.Fatal("Failed to select audio option: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	// Get Current active node
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object")
	}
	audioDeviceName, _, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}

	// Wait for audio node to switch
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Error while waiting for audio node to switch: ", err)
	}

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}
}
