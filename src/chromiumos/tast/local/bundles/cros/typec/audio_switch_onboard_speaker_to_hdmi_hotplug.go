// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioSwitchOnboardSpeakerToHDMIHotplug,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies local audio playback through default app, switch audio playback between internal-speaker and HDMI display",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:typec"},
		Vars:         []string{"typec.cSwitchPort", "typec.domainIP"},
		HardwareDeps: hwdep.D(hwdep.Speaker(), hwdep.InternalDisplay()),
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "quick",
			Val:  1,
		}, {
			Name:    "bronze",
			Val:     10,
			Timeout: 6 * time.Minute,
		}, {
			Name:    "silver",
			Val:     15,
			Timeout: 8 * time.Minute,
		}, {
			Name:    "gold",
			Val:     20,
			Timeout: 12 * time.Minute,
		}},
	})
}

// AudioSwitchOnboardSpeakerToHDMIHotplug performs typec HDMI external display hotplug
// and does audio switching from internal-speaker to HDMI display via quicksettings.
//
// AudioSwitchOnboardSpeakerToHDMIHotplug test requires the following H/W topology to run.
// DUT ------> C-Switch(device that performs hot plug-unplug) ----> External Type-C HDMI display.
func AudioSwitchOnboardSpeakerToHDMIHotplug(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Config file which contains expected values of USB4/TBT parameters.
	const testConfig = "test_config.json"
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to retrieve users Downloads path: ", err)
	}

	// Generate sine raw input file.
	rawFileName := "audioTestFile.raw"
	rawFilePath := filepath.Join(downloadsPath, rawFileName)
	rawFile := audio.TestRawData{
		Path:          rawFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      720,
	}
	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		s.Fatal("Failed to generate audio test data: ", err)
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "audioTestFile.wav"
	wavFile := filepath.Join(downloadsPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}

	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", wavFileName, err)
	}

	// Closing the audio player.
	defer kb.Accel(cleanupCtx, "Ctrl+W")

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	const cSwitchOFF = "0"
	defer func(ctx context.Context) {
		s.Log("Performing cleanup")
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port at cleanup: ", err)
		}
		if err := cswitch.CloseSession(ctx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	playPauseButton := nodewith.Name("Toggle play pause").Role(role.Button)
	ui := uiauto.New(tconn)
	infoBeforePause, err := ui.Info(ctx, playPauseButton)
	if err != nil {
		s.Fatal("Failed to get UI node info during audio playback: ", err)
	}

	iter := s.Param().(int)
	for i := 1; i <= iter; i++ {
		s.Logf("Iteration: %d/%d", i, iter)
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
			s.Fatal("Failed to enable c-switch port: ", err)
		}

		if err := performAudioSwitching(ctx, ui, tconn, cras, playPauseButton, infoBeforePause); err != nil {
			s.Fatal("Failed to perform audio switching: ", err)
		}

		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}
	}
}

// verifyFirstRunningDevice verifies whether audio is routing through audioDeviceName or not.
func verifyFirstRunningDevice(ctx context.Context, audioDeviceName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if audioDeviceName != devName {
			return errors.Wrapf(err, "unexpected audio node: got %q; want %q", devName, audioDeviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// selectedAudioNodeViaUI selects audio node from quick-settings, returns audioDeviceName and audioDeviceType.
func selectedAudioNodeViaUI(ctx context.Context, cras *audio.Cras, tconn *chrome.TestConn, audioNodeUIElement string) (string, string, error) {
	// Select output device.
	if err := quicksettings.Show(ctx, tconn); err != nil {
		return "", "", errors.Wrap(err, "failed to show Quick Settings")
	}
	if err := quicksettings.SelectAudioOption(ctx, tconn, audioNodeUIElement); err != nil {
		return "", "", errors.Wrap(err, "failed to select audio option")
	}
	// Get Current active node.
	audioDeviceName, audioDeviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get the selected audio device")
	}
	if err := quicksettings.Hide(ctx, tconn); err != nil {
		return "", "", errors.Wrap(err, "failed to hide quick settings")
	}
	return audioDeviceName, audioDeviceType, nil
}

// typecHDMIDisplayDetection checks for detection of typec HDMI external display.
func typecHDMIDisplayDetection(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var displayName string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		displayInfo, err := display.GetInfo(ctx, tconn)
		if err != nil {
			return errors.Wrap(err, "failed to get external display name")
		}
		if len(displayInfo) < 2 {
			return errors.New("failed to find external display")
		}
		displayName = displayInfo[1].Name

		const displayInfoFile = "/sys/kernel/debug/dri/0/i915_display_info"
		out, err := ioutil.ReadFile(displayInfoFile)
		if err != nil {
			return errors.Wrap(err, "failed to read i915_display_info file")
		}
		displayInfoRe := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
		matches := displayInfoRe.FindAllString(string(out), -1)
		if len(matches) != 1 {
			return errors.New("failed to check external display info")
		}
		typecHDMIRe := regexp.MustCompile(`.*DP branch device present.*yes\n.*Type.*HDMI`)
		if !typecHDMIRe.MatchString(string(out)) {
			return errors.New("failed to detect external typec HDMI display")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return "", errors.Wrap(err, "timeout to get external display info")
	}
	return displayName, nil
}

// resumeAudioPlayback will resumes the audio playback.
func resumeAudioPlayback(ctx context.Context, ui *uiauto.Context, playPauseButton *nodewith.Finder, infoBeforePause *uiauto.NodeInfo) error {
	infoAtPause, err := ui.Info(ctx, playPauseButton)
	if err != nil {
		return errors.Wrap(err, "failed to get UI playPauseButton node info")
	}
	if infoBeforePause != infoAtPause {
		testing.ContextLog(ctx, "Resuming audio play")
		if err := ui.LeftClick(playPauseButton)(ctx); err != nil {
			return errors.Wrap(err, "failed to play audio")
		}
	}
	return nil
}

func performAudioSwitching(ctx context.Context, ui *uiauto.Context, tconn *chrome.TestConn, cras *audio.Cras, playPauseButton *nodewith.Finder, infoBeforePause *uiauto.NodeInfo) error {
	// Check whether external typec HDMI display is detected.
	displayName, err := typecHDMIDisplayDetection(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to detect connected typec HDMI display")
	}

	// From quick-settigs select internal-speaker as output audio node.
	audioNodeInternalSpeaker := "Speaker (internal)"
	audioDeviceName, audioDeviceType, err := selectedAudioNodeViaUI(ctx, cras, tconn, audioNodeInternalSpeaker)
	if err != nil {
		return errors.Wrapf(err, "failed to select %q as audio output node", audioDeviceType)
	}

	// Check whether audio routing device is internal-speaker.
	if err := verifyFirstRunningDevice(ctx, audioDeviceName); err != nil {
		if err := resumeAudioPlayback(ctx, ui, playPauseButton, infoBeforePause); err != nil {
			return errors.Wrap(err, "failed to resume audio playback after selecting internal-speaker")
		}
		if err := verifyFirstRunningDevice(ctx, audioDeviceName); err != nil {
			return errors.Wrapf(err, "failed to route audio through %q audio device", audioDeviceName)
		}
	}

	// From quick-settigs select external HDMI as output audio node.
	audioNodeHDMI := displayName + " (HDMI/DP)"
	audioDeviceName, audioDeviceType, err = selectedAudioNodeViaUI(ctx, cras, tconn, audioNodeHDMI)
	if err != nil {
		return errors.Wrapf(err, "failed to select %q as audio output node", audioDeviceType)
	}

	// Check whether audio routing device is external HDMI.
	if err := verifyFirstRunningDevice(ctx, audioDeviceName); err != nil {
		if err := resumeAudioPlayback(ctx, ui, playPauseButton, infoBeforePause); err != nil {
			return errors.Wrap(err, "failed to resume audio playback after selecting exteranl HDMI")
		}
		if err := verifyFirstRunningDevice(ctx, audioDeviceName); err != nil {
			return errors.Wrapf(err, "failed to route audio through %q audio device", audioDeviceType)
		}
	}
	return nil
}
