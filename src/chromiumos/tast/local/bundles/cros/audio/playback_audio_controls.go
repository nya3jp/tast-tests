// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/audio/audionode"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
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

		// Params: []testing.Param{
		// 	{
		// 		Name:              "",
		// 		Val: true,
		// 		ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("dru", "dumo")),
		// 	},
		// 	{
		// 		Name:              "no_play_queue_button",
		// 		Val: false,
		// 		ExtraHardwareDeps: hwdep.D(hwdep.Model("dru", "dumo")),
		// 	},
		// },
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

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up capture (aloop) module.
	unload, err := audio.LoadAloop(ctx)
	if err != nil {
		s.Fatal("Failed to load ALSA loopback module: ", err)
	}

	defer func(ctx context.Context) {
		// Wait for no stream before unloading aloop as unloading while there is a stream
		// will cause the stream in ARC to be in an invalid state.
		if err := crastestclient.WaitForNoStream(ctx, 5*time.Second); err != nil {
			s.Error("Wait for no stream error: ", err)
		}
		unload(ctx)
	}(cleanupCtx)

	// Select ALSA loopback output and input nodes as active nodes by UI.
	// Call Hide() and Show() to reset the Quick Settings menu first.
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		s.Fatal("Failed to hide Quick Settings menu: ", err)
	}
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings menu: ", err)
	}
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Loopback Playback"); err != nil {
		s.Fatal("Failed to select ALSA loopback output: ", err)
	}

	// Ensure landscape orientation. Gallery app has different UI if its size is
	// in portrait, and the play queue buttons don't exist.
	orientation, err := display.GetOrientation(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain the orientation info: ", err)
	}
	displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to obtain primary display info: ", err)
	}
	if orientation.Type == display.OrientationPortraitPrimary {
		if err = display.SetDisplayRotationSync(ctx, tconn, displayInfo.ID, display.Rotate90); err != nil {
			s.Fatal("Failed to rotate display: ", err)
		}
		defer display.SetDisplayRotationSync(cleanupCtx, tconn, displayInfo.ID, display.Rotate0)
	}

	// After selecting Loopback Playback, SelectAudioOption() sometimes detected that audio setting
	// is still opened while it is actually fading out, and failed to select Loopback Capture.
	// Call Hide() and Show() to reset the quicksettings menu first.
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		s.Fatal("Failed to hide Quick Settings menu: ", err)
	}
	if err := quicksettings.Show(ctx, tconn); err != nil {
		s.Fatal("Failed to show Quick Settings menu: ", err)
	}
	if err := quicksettings.SelectAudioOption(ctx, tconn, "Loopback Capture"); err != nil {
		s.Fatal("Failed to select ALSA loopback input: ", err)
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

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Close audio player window as cleanup.
	defer kb.Accel(ctx, "Ctrl+W")

	if err := generateWAVAudioFile(ctx, rawFilePath1, wavFilePath1); err != nil {
		s.Fatal("Failed to create WAV audio file: ", err)
	}

	if err := generateWAVAudioFile(ctx, rawFilePath2, wavFilePath2); err != nil {
		s.Fatal("Failed to create WAV audio file: ", err)
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)

	iter := 3
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)

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

		// Verify whether audio is playing or not.
		if _, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream); err != nil {
			s.Fatal("Failed to play audio: ", err)
		}

		// Maximize window on first iteration, to ensure all buttons exist.
		if i == 1 {
			window, err := ash.FindWindow(ctx, tconn, func(w *ash.Window) bool {
				return strings.HasPrefix(w.Title, "Gallery - ")
			})
			if err != nil {
				s.Fatal("Failed to find the Gallery app window: ", err)
			}
			if err := ash.SetWindowStateAndWait(ctx, tconn, window.ID, ash.WindowStateMaximized); err != nil {
				s.Fatal("Failed to maximize the Gallery app window: ", err)
			}
		}

		if err := performAudioControls(ctx, ui, kb, wavFileName1, wavFileName2); err != nil {
			s.Fatal("Failed to preform audio player various controls: ", err)
		}

		// Verify whether audio is playing or not.
		if _, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream); err != nil {
			s.Fatal("Failed to play audio: ", err)
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

	if err := performPlayQueueControls(ctx, ui); err != nil {
		return errors.Wrap(err, "failed to perform play queue controls")
	}

	return nil
}

// performPlayQueueControls performs various play queue controls.
func performPlayQueueControls(ctx context.Context, ui *uiauto.Context) error {

	collapsePlayQueueButton := nodewith.Name("Collapse play queue").Role(role.Button)
	expandPlayQueueButton := nodewith.Name("Expand play queue").Role(role.Button)

	// Check the initial state of the play queue to determine whether to collapse
	// or expand first.
	if isFound, err := ui.IsNodeFound(ctx, collapsePlayQueueButton); err != nil {
		return errors.Wrap(err, "failed to check if 'Collapse play queue' button is found")
	} else if !isFound {
		if err := audioPlayerControls(ctx, ui, expandPlayQueueButton); err != nil {
			return errors.Wrap(err, "failed to expand playlist")
		}
		if err := audioPlayerControls(ctx, ui, collapsePlayQueueButton); err != nil {
			return errors.Wrap(err, "failed to collapse playlist")
		}
	} else {
		if err := audioPlayerControls(ctx, ui, collapsePlayQueueButton); err != nil {
			return errors.Wrap(err, "failed to collapse playlist")
		}
		if err := audioPlayerControls(ctx, ui, expandPlayQueueButton); err != nil {
			return errors.Wrap(err, "failed to expand playlist")
		}
	}

	return nil
}
