// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioPlayBtHeadsetManual,
		Desc:         "Verifies audio playback over BT headset",
		Contacts:     []string{"intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"bluetooth.btHeadset"},
		Fixture:      "chromeLoggedIn",
	})
}

var expectedAudioNode = "BLUETOOTH"

// AudioPlayBtHeadsetManual plays audio file over BT Headset.
// Manual step: bluetooth.btHeadset bluetooth device has to be set to pairing mode before executing test-script.
func AudioPlayBtHeadsetManual(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	btHeadset := s.RequiredVar("bluetooth.btHeadset")

	adapters, err := bluetooth.Adapters(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth adapters: ", err)
	}
	adapter := adapters[0]

	// Turn on bluetooth adapter.
	isPowered, err := adapter.Powered(ctx)
	if err != nil {
		s.Fatal("Failed to get powered property value: ", err)
	}
	if !isPowered {
		if err := adapter.SetPowered(ctx, true); err != nil {
			s.Fatal("Failed to turn on bluetooth adapter: ", err)
		}
	}

	if err := adapter.StartDiscovery(ctx); err != nil {
		s.Fatal("Failed to enable discovery: ", err)
	}

	// Waits for a specific BT device to be found.
	var btDevice *bluetooth.Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDevice, err = bluetooth.DeviceByAlias(ctx, btHeadset)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for BT Headset: ", err)
	}

	// Pair BT Device.
	isPaired, err := btDevice.Paired(ctx)
	if !isPaired {
		if err := btDevice.Pair(ctx); err != nil {
			s.Fatal("Failed to pair bluetooth device: ", err)
		}
	}

	if err := bluetooth.DisconnectAllDevices(ctx); err != nil {
		s.Fatal("Failed to disconnect the devices: ", err)
	}

	// Get connected status of BT device and connect if not already connected.
	isConnected, err := btDevice.Connected(ctx)
	if err != nil {
		s.Fatal("Failed to get BT connected status: ", err)
	}
	if !isConnected {
		if err := btDevice.Connect(ctx); err != nil {
			s.Fatal("Failed to connect bluetooth device: ", err)
		}
	}

	// Disconnect BT device.
	defer btDevice.Disconnect(ctx)

	if err := audioPlay(ctx, tconn); err != nil {
		s.Fatal("Failed to play audio: ", err)
	}
}

// audioPlay generates wav audio file, play using default player
// and verify audio routing through BT device.
func audioPlay(ctx context.Context, tconn *chrome.TestConn) error {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Cras object")
	}

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

	// Generate sine raw input file that lasts 30 seconds.
	rawFileName := "AudioFile.raw"
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
		return errors.Wrap(err, "failed to generate audio test data")
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "AudioFile.wav"
	wavFile := filepath.Join(filesapp.DownloadPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		return errors.Wrap(err, "failed to convert raw to wav")
	}
	defer os.Remove(wavFile)

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to find keyboard")
	}
	defer kb.Close()

	defer func() {
		// Closing the audio player.
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			testing.ContextLog(ctx, "Failed to close Audio player: ", err)
		}
	}()

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
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		return errors.Wrap(err, "timeout waiting for BT Headset")
	}
	return nil
}
