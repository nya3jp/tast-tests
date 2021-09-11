// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
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
		Func:         PlaybackVolumeControl,
		Desc:         "Play local audio file through default app and change the volume using keyboard keys",
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
		}},
	})
}

// PlaybackVolumeControl generates audio file plays it through default audio player.
func PlaybackVolumeControl(ctx context.Context, s *testing.State) {
	const (
		storageType  = "Downloads"
		audioRate    = 48000
		audioChannel = 2
		duration     = 30
	)

	expectedAudioNode := s.Param().(string)
	cr := s.PreValue().(*chrome.Chrome)

	// Generate sine raw input file that lasts 30 seconds.
	rawFileName := "30SEC.raw"
	rawFilePath := filepath.Join(filesapp.DownloadPath, rawFileName)
	rawFile := audio.TestRawData{
		Path:          rawFilePath,
		BitsPerSample: 16,
		Channels:      audioChannel,
		Rate:          audioRate,
		Frequencies:   []int{440, 440},
		Volume:        0, // Generate audio file with volume 0 to avoid noisiness in lab.
		Duration:      duration,
	}
	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		s.Fatal("Failed to generate audio test data: ", err)
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "30SEC.wav"
	wavFile := filepath.Join(filesapp.DownloadPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, audioRate, audioChannel); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
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
	s.Logf("Selected audio device name: %q; audio device type: %q", audioDeviceName, audioDeviceType)

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}

	vh, err := audionode.NewVolumeHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create the volumeHelper: ", err)
	}
	originalVolume, err := vh.ActiveNodeVolume(ctx)
	defer vh.SetVolume(ctx, originalVolume)

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		s.Fatal("Failed to obtain the top-row layout: ", err)
	}

	isMuted := func() bool {
		dump, err := testexec.CommandContext(ctx, "sh", "-c", "cras_test_client --dump_server_info | grep muted").Output()
		if err != nil {
			s.Errorf("Failed to dump server info: %s", err)
		}
		muted := strings.TrimSpace(string(dump[strings.LastIndex(string(dump), ":")+1:]))
		return muted == "Muted"
	}

	// Press mute key and unmute by pressing Volume up key
	if err = kb.Accel(ctx, topRow.VolumeMute); err != nil {
		s.Fatal(`Failed to press "Mute": `, err)
	}
	if !isMuted() {
		s.Fatal("Failed to mute the audio")
	}

	if err = kb.Accel(ctx, topRow.VolumeUp); err != nil {
		s.Fatal(`Failed to press "VolumeUp": `, err)
	}

	if isMuted() {
		s.Fatal("Failed to unmute the audio")
	}

	// Decrease volume to 0 and verify
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			s.Fatal("Failed to get volume: ", err)
		}
		if volume == 0 {
			break
		}
		if err := vh.VerifyVolumeChanged(ctx, func() error {
			return kb.Accel(ctx, topRow.VolumeDown)
		}); err != nil {
			s.Fatal(`Failed to change volume after pressing "VolumeDown": `, err)
		}
	}

	// Increase volume to 100 and verify
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			s.Fatal("Failed to get volume: ", err)
		}
		if volume == 100 {
			break
		}
		if err := vh.VerifyVolumeChanged(ctx, func() error {
			return kb.Accel(ctx, topRow.VolumeUp)
		}); err != nil {
			s.Fatal(`Failed to change volume after pressing "VolumeUp": `, err)
		}
	}
}
