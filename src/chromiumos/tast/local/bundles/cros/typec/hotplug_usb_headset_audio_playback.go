// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HotplugUSBHeadsetAudioPlayback,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies audio playback with USB type-C headset hotplug",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"volumeDown.txt", "volumeUp.txt"},
		Vars:         []string{"typec.cSwitchPort", "typec.domainIP"},
		// TODO(b/207569436): Define hardware dependency and get rid of hard-coding the models.
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel", "redrix", "brya")),
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
}

// HotplugUSBHeadsetAudioPlayback test requires the following H/W topology to run.
// DUT ------> C-Switch(device that performs hot plug-unplug) ----> USB Headset.
func HotplugUSBHeadsetAudioPlayback(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	// Config file which contains expected values of USB4/TBT parameters.
	const testConfig = "test_config.json"
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

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
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}

		// Closing the audio player.
		ui := uiauto.New(tconn)
		nowPlayingText := nodewith.Name("Now playing").Role(role.StaticText)
		if err := ui.Exists(nowPlayingText)(ctx); err == nil {
			audioPlayerTitle := "Gallery - AudioFile.wav"
			audioPlayerWindow, err := ash.BringWindowToForeground(ctx, tconn, audioPlayerTitle)
			if err != nil {
				s.Error("Failed to bring audio player to foreground: ", err)
			}
			if err := audioPlayerWindow.CloseWindow(ctx, tconn); err != nil {
				s.Error("Failed to close autio player: ", err)
			}
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	audioOutputNode := "USB"
	if err := usbHeadsetDetection(ctx, audioOutputNode); err != nil {
		s.Fatal("Failed to detect USB headset after plug: ", err)
	}

	if err := audioPlay(ctx, tconn); err != nil {
		s.Fatal("Failed to play audio file: ", err)
	}

	const (
		usbVolumeDownFile = "volumeDown.txt"
		usbVolumeUpFile   = "volumeUp.txt"
	)

	infoBeforeVolumeDown, err := dutVolumeInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get DUT volume info before volume down: ", err)
	}

	if err := usbHeadsetEvents(ctx, s.DataPath(usbVolumeDownFile)); err != nil {
		s.Fatal("Failed to perform volume-down with USB headset volumeDown button: ", err)
	}

	infoAfterVolumeDown, err := dutVolumeInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get DUT volume info after volume down: ", err)
	}

	if infoBeforeVolumeDown == infoAfterVolumeDown {
		s.Fatal("Failed to volume down with USB headset button press")
	}

	if err := usbHeadsetEvents(ctx, s.DataPath(usbVolumeUpFile)); err != nil {
		s.Fatal("Failed to perform volume-up with USB headset volumeUp button: ", err)
	}

	infoAfterVolumeUp, err := dutVolumeInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get DUT volume info after volume up: ", err)
	}

	if infoAfterVolumeDown == infoAfterVolumeUp {
		s.Fatal("Failed to volume up with USB headset button press")
	}

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	audioOutputNode = "INTERNAL_SPEAKER"
	if err := usbHeadsetDetection(ctx, audioOutputNode); err != nil {
		s.Fatal("Failed to detect INTERNAL_SPEAKER after unplug: ", err)
	}
}

// audioPlay generates wav audio file, play using default player
// and verify audio routing through BT device.
func audioPlay(ctx context.Context, tconn *chrome.TestConn) error {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Cras object")
	}

	const expectedAudioNode = "USB"

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the selected audio device")
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

	// Generate sine raw input file that lasts 60 seconds.
	rawFileName := "AudioFile.raw"
	rawFilePath := filepath.Join(filesapp.DownloadPath, rawFileName)
	rawFile := audio.TestRawData{
		Path:          rawFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      60,
	}
	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		return errors.Wrap(err, "failed to generate audio test data")
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "AudioFile.wav"
	wavFile := filepath.Join(filesapp.DownloadPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		return errors.Wrap(err, "failed to convert raw to wav")
	}
	defer os.Remove(wavFile)

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch the Files App")
	}
	defer files.Close(ctx)

	if err := files.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "failed to open Downloads folder in files app")
	}
	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		return errors.Wrapf(err, "failed to open the audio file %q", wavFileName)
	}

	// Verify whether audio is routing through BT device or not.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "timeout waiting for USB Headset")
	}
	return nil
}

// usbHeadsetDetection checks for connected USB headset detection.
func usbHeadsetDetection(ctx context.Context, audioOutputNode string) error {
	audioNodeRe := regexp.MustCompile(fmt.Sprintf(`.*yes.*%s.*\*`, audioOutputNode))
	return testing.Poll(ctx, func(ctx context.Context) error {
		crasOut, err := testexec.CommandContext(ctx, "cras_test_client").Output()
		if err != nil {
			return errors.Wrap(err, "failed to execute cras_test_client command")
		}
		if !audioNodeRe.MatchString(string(crasOut)) {
			return errors.New("audio output node is not USB Headset")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// usbHeadsetEvents will handle evtest events for connected USB speaker.
func usbHeadsetEvents(ctx context.Context, usbKeyFile string) error {
	usbKeyPlay := "evemu-play --insert-slot0 /dev/input/event"
	testing.ContextLogf(ctx, "Pressing USB headset Key : %s", strings.Split(filepath.Base(usbKeyFile), ".")[0])
	out, _ := exec.Command("evtest").CombinedOutput()
	re := regexp.MustCompile(`(?i)/dev/input/event([0-9]+):.*(USB).*`)
	result := re.FindStringSubmatch(string(out))
	usbHSEventNum := ""
	if len(result) > 0 {
		usbHSEventNum = result[1]
	} else {
		return errors.New("USB headset not found in evtest command output")
	}
	testing.ContextLogf(ctx, "USB headset mount point:/dev/input/event%s", usbHSEventNum)
	if err := testexec.CommandContext(ctx, "bash", "-c", usbKeyPlay+usbHSEventNum+" < "+usbKeyFile).Run(); err != nil {
		return errors.Wrap(err, "failed to play USB headset event")
	}
	return nil
}

// dutVolumeInfo returns current DUT volume information.
func dutVolumeInfo(ctx context.Context) (string, error) {
	amixer, err := testexec.CommandContext(ctx, "amixer", "get", "Master").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to execute amixer command")
	}
	return string(amixer), nil
}
