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
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
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

// isMuted verifies if audio is muted.
func isMuted(ctx context.Context) (bool, error) {
	dump, err := testexec.CommandContext(ctx, "sh", "-c", "cras_test_client --dump_server_info | grep muted").Output()
	if err != nil {
		return false, errors.Wrap(err, "failed to dump server info")
	}
	muted := strings.TrimSpace(string(dump[strings.LastIndex(string(dump), ":")+1:]))
	return strings.Contains(muted, "Muted"), nil
}

// isPlayingOnDevice will check if audio is playing through expected audio node.
func isPlayingOnDevice(ctx context.Context, audioDeviceName string) error {
	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		return errors.Wrap(err, "failed to detect running output device")
	}
	if audioDeviceName != devName {
		return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, audioDeviceName)
	}
	return nil
}

func muteUnmuteVolume(ctx context.Context, tconn *chrome.TestConn) error {
	mutedButton := nodewith.Name("Toggle Volume. Volume is muted.").Role(role.ToggleButton)
	unmutedButton := nodewith.Name("Toggle Volume. Volume is on, toggling will mute audio.").Role(role.ToggleButton)

	ui := uiauto.New(tconn)

	// Muting the audio.
	if err := ui.WithTimeout(1 * time.Second).LeftClick(unmutedButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the audio toggle")
	}

	unMute, err := isMuted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check audio mute status")
	}
	if !unMute {
		return errors.New("failed to mute the audio")
	}

	// Unmuting the audio.
	if err := ui.WithTimeout(1 * time.Second).LeftClick(mutedButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click the audio toggle")
	}

	mute, err := isMuted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check audio mute status")
	}
	if mute {
		return errors.New("failed to mute the audio")
	}
	return nil
}

func incDecVolume(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	currVal, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeMicGain)
	if err != nil {
		return errors.Wrap(err, "failed initial value check for mic gain slider")
	}
	testing.ContextLogf(ctx, "Initial mic gain slider value: %d", currVal)

	decrease, err := quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
	if err != nil {
		return errors.Wrap(err, "failed to decrease mic gain slider")
	}
	testing.ContextLogf(ctx, "Decreased mic gain slider value: %d", decrease)

	if decrease >= currVal {
		return errors.Errorf("failed to decrease mic gain slider value; initial: %d, decrease: %d", currVal, decrease)
	}

	increase, err := quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeMicGain)
	if err != nil {
		return errors.Wrap(err, "failed to increase mic gain slider")
	}
	testing.ContextLogf(ctx, "Increased mic gain slider value: %d", increase)

	if currVal != increase {
		return errors.Errorf("failed to increase mic gain slider value; initial: %d, increased: %d", currVal, increase)
	}
	return nil
}

// HeadphoneVolumeSlider verifies volume slider, mute/unmute works fine for audio playback in 3.5MM Jack in lockscreen.
func HeadphoneVolumeSlider(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

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
		Duration:      20,
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

	// Check if audio is playing on device or not.
	if err := isPlayingOnDevice(ctx, audioDeviceName); err != nil {
		s.Fatal("Failed while verifying audio playing on device: ", err)
	}

	// Lock screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		s.Fatalf("Waiting for the screen to be locked failed: %v (last status %+v)", err, st)
	}

	defer func() {
		// Unlocking the screen.
		if err := lockscreen.Unlock(ctx, tconn); err != nil {
			s.Fatal("Failed to unlock the screen: ", err)
		}
		// Closing the audio player.
		if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}()

	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings: ", err)
	}
	defer quicksettings.Hide(ctx, tconn)

	if err := muteUnmuteVolume(ctx, tconn); err != nil {
		s.Fatal("Failed to mute/unmute volume: ", err)
	}

	if err := incDecVolume(ctx, tconn, kb); err != nil {
		s.Fatal("Failed to increase/decrease volume slider: ", err)
	}
}
