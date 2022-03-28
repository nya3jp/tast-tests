// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HeadphoneVolumeSlider,
		Desc:         "System volume slider works fine for audio playback in 3.5MM headset in lockscreen",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		LacrosStatus: testing.LacrosVariantUnneeded,
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// HeadphoneVolumeSlider verifies volume slider, mute/unmute works fine for audio playback in 3.5MM Jack in lockscreen.
func HeadphoneVolumeSlider(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

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
	// Copy generated audio file to Downloads folder.
	wavFile := filepath.Join(filesapp.DownloadPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to Launch the Files App: ", err)
	}
	defer files.Close(ctx)

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder: ", err)
	}
	expectedAudioNode := "HEADPHONE"
	audioDeviceName, err := audionode.SetAudioNode(ctx, expectedAudioNode)
	if err != nil {
		s.Fatal("Failed to set the Audio node: ", err)
	}
	s.Logf("Selected audio device name: %s", audioDeviceName)

	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		s.Fatalf("Failed clicking %q: %v", wavFileName, err)
	}

	// Sample time for the audio to play for 5 seconds.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed while waiting during sample time: ", err)
	}

	//Lock screen
	if err := quicksettings.LockScreen(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	mutedFinder := nodewith.Name("Toggle Volume. Volume is muted.").Role(role.ToggleButton)
	unmutedFinder := nodewith.Name("Toggle Volume. Volume is on, toggling will mute audio.").Role(role.ToggleButton)

	ui := uiauto.New(tconn)

	// Muting the audio.
	if err := ui.WithTimeout(1 * time.Second).LeftClick(unmutedFinder)(ctx); err != nil {
		s.Fatal("Failed to click the audio toggle: ", err)
	}

	isMuted := func() bool {
		dump, err := testexec.CommandContext(ctx, "sh", "-c", "cras_test_client --dump_server_info | grep muted").Output()
		if err != nil {
			s.Errorf("Failed to dump server info: %s", err)
		}
		muted := strings.TrimSpace(string(dump[strings.LastIndex(string(dump), ":")+1:]))
		return muted == "Muted"
	}

	if !isMuted() {
		s.Fatal("Failed to mute the audio")
	}
	// Un muting the audio.
	if err := ui.WithTimeout(1 * time.Second).LeftClick(mutedFinder)(ctx); err != nil {
		s.Fatal("Failed to click the audio toggle: ", err)
	}

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}
	if audioDeviceName != devName {
		s.Fatalf("Failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	decrease, err := quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
	if err != nil {
		s.Fatal("Failed to decrease mic gain slider: ", err)
	}
	s.Log("Decreased mic gain slider value: ", decrease)

	increase, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
	if err != nil {
		s.Fatal("Failed to increase mic gain slider: ", err)
	}
	s.Log("Increased mic gain slider value: ", increase)

}
