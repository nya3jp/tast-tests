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
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/filesapp"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalAudioPlayback,
		Desc:         "Play local audio file through default app and check if the audio is routing through expected device",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Params: []testing.Param{{
			Name:      "internal_speaker",
			ExtraAttr: []string{"group:mainline", "informational"},
			Val:       "INTERNAL_SPEAKER",
		}, {
			Name: "headphone",
			Val:  "HEADPHONE",
		}, {
			Name: "usb_speaker",
			Val:  "USB",
		}},
	})
}

// LocalAudioPlayback generates audio file and plays it through default audio player.
func LocalAudioPlayback(ctx context.Context, s *testing.State) {
	expectedAudioNode := s.Param().(string)
	storageType := "Downloads"
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

	kb, err := input.Keyboard(ctx)
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
	defer files.Release(ctx)
	if err := files.OpenDir(ctx, storageType, "Files - "+storageType); err != nil {
		s.Fatalf("Failed to open %v folder in files app: %v", storageType, err)
	}
	if err := files.OpenFile(ctx, wavFileName); err != nil {
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

	audioDeviceName, audioDeviceType, err := audionode.SetAudioNode(ctx, expectedAudioNode)
	if err != nil {
		s.Fatal("Failed to set the Audio node: ", err)
	}
	s.Logf("Selected audio device name: %s", audioDeviceName)
	s.Logf("Selected audio device type: %s", audioDeviceType)

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}
}
