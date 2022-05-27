// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlaybackAudioControls,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies local audio playback through default app and perform various audio player controls",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeLoggedIn",
		HardwareDeps: hwdep.D(hwdep.Speaker()),
	})
}

func PlaybackAudioControls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users Downloads path: ", err)
	}

	// First audio file name and path variables.
	rawFileName1 := "audioFile1.raw"
	rawFilePath1 := filepath.Join(downloadsPath, rawFileName1)
	wavFileName1 := "audioFile1.wav"
	wavFilePath1 := filepath.Join(downloadsPath, wavFileName1)
	defer os.Remove(wavFilePath1)

	// Second audio file name and path variables.
	rawFileName2 := "audioFile2.raw"
	rawFilePath2 := filepath.Join(downloadsPath, rawFileName2)
	wavFileName2 := "audioFile2.wav"
	wavFilePath2 := filepath.Join(downloadsPath, wavFileName2)
	defer os.Remove(wavFilePath2)

	expectedAudioNode := "INTERNAL_SPEAKER"

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Close audio player window as cleanup.
	defer kb.Accel(ctx, "Ctrl+W")

	// Get Current active node.
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	var deviceName, deviceType string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get current audio output device info.
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device: ", err)
		}
		if deviceType != expectedAudioNode {
			if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
				return errors.Wrapf(err, "failed to select active device %s", expectedAudioNode)
			}
			deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to get the selected audio device")
			}
			if deviceType != expectedAudioNode {
				return errors.Wrapf(err, "failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Failed to set audio node: ", err)
	}

	if err := generateWAVAudioFile(ctx, rawFilePath1, wavFilePath1); err != nil {
		s.Fatal("Failed to create WAV audio file: ", err)
	}

	if err := generateWAVAudioFile(ctx, rawFilePath2, wavFilePath2); err != nil {
		s.Fatal("Failed to create WAV audio file: ", err)
	}

	iter := 3
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)
		files, err := filesapp.Launch(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to launch the Files App: ", err)
		}
		defer files.Close(ctx)

		if err := files.OpenDownloads()(ctx); err != nil {
			s.Fatal("Failed to open Downloads folder in files app: ", err)
		}

		ui := uiauto.New(tconn)
		fileNameTitleButton := nodewith.Name("Name").Role(role.Button)
		if err := audioPlayerControls(ctx, ui, fileNameTitleButton); err != nil {
			s.Fatal("Failed to click on file name title button: ", err)
		}

		if err := kb.Accel(ctx, "Ctrl+A"); err != nil {
			s.Fatal("Failed to select all files: ", err)
		}

		filesSelectedText := nodewith.Name("2 files selected").Role(role.StaticText)
		if err := ui.WaitForLocation(filesSelectedText)(ctx); err != nil {
			s.Fatal("Failed to wait for files selected UI text: ", err)
		}

		openButton := nodewith.Name("Open").Role(role.Button)
		if err := ui.WaitForLocation(openButton)(ctx); err != nil {
			s.Fatal("Failed to wait for file open button: ", err)
		}

		if err := ui.LeftClick(openButton)(ctx); err != nil {
			s.Fatal("Failed to left click open button: ", err)
		}

		// Verify whether audio is routing through internal-speaker or not.
		if err := verifyFirstRunningDevice(ctx, deviceName); err != nil {
			s.Fatal("Failed to route audio through onboard speaker: ", err)
		}

		if err := performAudioControls(ctx, ui, kb, wavFileName1, wavFileName2); err != nil {
			s.Fatal("Failed to preform audio player various controls: ", err)
		}

		// Verify whether audio is routing through internal-speaker or not.
		if err := verifyFirstRunningDevice(ctx, deviceName); err != nil {
			s.Fatal("Failed to route audio through onboard speaker: ", err)
		}

		// Closing the audio player.
		if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Fatal("Failed to close audio player: ", err)
		}
	}
}

// generateWAVAudioFile generates WAV audio file in given wavFilePath with rawFilePath.
func generateWAVAudioFile(ctx context.Context, rawFilePath, wavFilePath string) error {
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
		return errors.Wrap(err, "failed to generate audio test data")
	}
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFilePath, 48000, 2); err != nil {
		return errors.Wrap(err, "failed to convert raw to wav")
	}
	if err := os.Remove(rawFile.Path); err != nil {
		return errors.Wrap(err, "failed to remove audio raw file")
	}
	return nil
}

// verifyFirstRunningDevice verifies whether audio is running through deviceName.
func verifyFirstRunningDevice(ctx context.Context, deviceName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Errorf("failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond})
}

// audioPlayerControls performs various controls of audio player during playback.
func audioPlayerControls(ctx context.Context, ui *uiauto.Context, nodeWithFinder *nodewith.Finder) error {
	if err := ui.WaitForLocation(nodeWithFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for element location")
	}
	if err := ui.LeftClick(nodeWithFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to left click the element")
	}
	return nil
}

// presentPlayingAudioFile checks for current audio file playing window.
func presentPlayingAudioFile(ctx context.Context, ui *uiauto.Context, audioFileName string) error {
	nodeWithFinder := nodewith.Name("Gallery - " + audioFileName).Role(role.Window)
	if err := ui.WaitForLocation(nodeWithFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for element location")
	}
	return nil
}

// performVolumeControls performs volume controls through keyboard keypress.
func performVolumeControls(ctx context.Context, kb *input.KeyboardEventWriter) error {
	vh, err := audionode.NewVolumeHelper(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the volumeHelper")
	}
	originalVolume, err := vh.ActiveNodeVolume(ctx)
	defer vh.SetVolume(ctx, originalVolume)

	topRow, err := input.KeyboardTopRowLayout(ctx, kb)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}

	testing.ContextLog(ctx, "Press mute key and unmute by pressing Volume up key")
	if err = kb.Accel(ctx, topRow.VolumeMute); err != nil {
		return errors.Wrap(err, "failed to press 'Mute'")
	}

	audioVh, err := audio.NewVolumeHelper(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create the volumeHelper")
	}

	muted, err := audioVh.IsMuted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check audio mute status")
	}

	if !muted {
		return errors.New("failed to mute the audio")
	}

	if err = kb.Accel(ctx, topRow.VolumeUp); err != nil {
		return errors.Wrap(err, "failed to press 'VolumeUp'")
	}

	muted, err = audioVh.IsMuted(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check audio mute status after pressing volumeup")
	}

	if muted {
		return errors.New("failed to unmute the audio")
	}

	testing.ContextLog(ctx, "Decrease volume to 0 and verify for every key press")
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get volume during volume decrease")
		}
		if volume == 0 {
			break
		}
		if err := vh.VerifyVolumeChanged(ctx, func() error {
			return kb.Accel(ctx, topRow.VolumeDown)
		}); err != nil {
			return errors.Wrap(err, "failed to change volume after pressing 'VolumeDown'")
		}
	}

	testing.ContextLog(ctx, "Increase volume to 100 and verify for every key press")
	for {
		volume, err := vh.ActiveNodeVolume(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get volume during volume increase")
		}
		if volume == 100 {
			break
		}
		if err := vh.VerifyVolumeChanged(ctx, func() error {
			return kb.Accel(ctx, topRow.VolumeUp)
		}); err != nil {
			return errors.Wrap(err, "failed to change volume after pressing 'VolumeUp'")
		}
	}
	return nil
}

// performAudioControls performs various audio player controls using UI and
// volume controls using keyboard.
func performAudioControls(ctx context.Context, ui *uiauto.Context, kb *input.KeyboardEventWriter, wavFileName1, wavFileName2 string) error {
	stepForwardButton := nodewith.Name("Step forward").Role(role.Button)
	if err := audioPlayerControls(ctx, ui, stepForwardButton); err != nil {
		return errors.Wrap(err, "failed to step forward audio playback")
	}

	stepBackwardButton := nodewith.Name("Step backward").Role(role.Button)
	if err := audioPlayerControls(ctx, ui, stepBackwardButton); err != nil {
		return errors.Wrap(err, "failed to step backward audio playback")
	}

	playPauseButton := nodewith.Name("Toggle play pause").Role(role.Button)
	infoBeforePause, err := ui.Info(ctx, playPauseButton)
	if err != nil {
		return errors.Wrap(err, "failed to get UI node info before pausing audio playback")
	}

	if err := audioPlayerControls(ctx, ui, playPauseButton); err != nil {
		return errors.Wrap(err, "failed to press audio 'Pause' button")
	}

	infoAfterPause, err := ui.Info(ctx, playPauseButton)
	if err != nil {
		return errors.Wrap(err, "failed to get UI node info after pausing audio playback")
	}

	if infoBeforePause == infoAfterPause {
		return errors.Wrap(err, "failed to pause audio playback")
	}

	if err := audioPlayerControls(ctx, ui, playPauseButton); err != nil {
		return errors.Wrap(err, "failed to press audio 'Play' button")
	}

	infoAfterPlay, err := ui.Info(ctx, playPauseButton)
	if err != nil {
		return errors.Wrap(err, "failed to get UI node info after playing audio playback")
	}

	if infoAfterPause == infoAfterPlay {
		return errors.Wrap(err, "failed to play audio playback")
	}

	nextAudioButton := nodewith.Name("Skip next").Role(role.Button)
	if err := audioPlayerControls(ctx, ui, nextAudioButton); err != nil {
		return errors.Wrap(err, "failed to skip to next audiofile")
	}

	if err := presentPlayingAudioFile(ctx, ui, wavFileName2); err != nil {
		return errors.Wrapf(err, "failed to skip next and play %s audio file", wavFileName2)
	}

	previuosAudioButton := nodewith.Name("Skip previous").Role(role.Button)
	if err := audioPlayerControls(ctx, ui, previuosAudioButton); err != nil {
		return errors.Wrap(err, "failed to skip to previous audiofile")
	}

	if err := presentPlayingAudioFile(ctx, ui, wavFileName1); err != nil {
		return errors.Wrapf(err, "failed to skip previous and play %s audio file", wavFileName1)
	}

	if err := performVolumeControls(ctx, kb); err != nil {
		return errors.Wrap(err, "failed to perform audio volume controls")
	}
	return nil
}
