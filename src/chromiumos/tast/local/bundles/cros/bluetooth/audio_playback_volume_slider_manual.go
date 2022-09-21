// Copyright 2022 The ChromiumOS Authors
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
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioPlaybackVolumeSliderManual,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "System volume slider works fine for audio playback on Bluetooth headset",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"bluetooth.btHeadset"},
		Fixture:      "chromeLoggedIn",
	})
}

// AudioPlaybackVolumeSliderManual tests the volume slider works fine for audio playback on Bluetooth headset.
func AudioPlaybackVolumeSliderManual(ctx context.Context, s *testing.State) {
	Headset := s.RequiredVar("bluetooth.btHeadset")
	cr := s.FixtValue().(*chrome.Chrome)

	expectedAudioNode := "BLUETOOTH"

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth adapters: ", err)
	}
	if len(adapters) != 1 {
		s.Fatalf("Failed: got %d adapters, want 1 adapter", len(adapters))
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
	var btDevice *bluez.Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDevice, err = bluez.DeviceByAlias(ctx, Headset)
		if err != nil {
			return errors.Wrap(err, "failed to find bluetooth device by alias name")
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

	if err := bluez.DisconnectAllDevices(ctx); err != nil {
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	// Generate sine raw input file that lasts 30 seconds.
	rawFileName := "AudioFile.raw"
	rawFilePath := filepath.Join(downloadsPath, rawFileName)
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

	wavFileName := "AudioFile.wav"
	wavFile := filepath.Join(downloadsPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal(err, "failed to get the selected audio device")
	}

	if deviceType != expectedAudioNode {
		if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
			s.Fatalf("Failed to select active device %s: %v", expectedAudioNode, err)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal(err, "failed to get the selected audio device")
		}
		if deviceType != expectedAudioNode {
			s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
		}
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)
	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}

	kb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kb.Close()

	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", wavFileName, err)
	}
	defer func() {
		if err := kb.Accel(ctx, "Ctrl+W"); err != nil {
			s.Error("Failed to close Audio player: ", err)
		}
	}()

	// Sample time for the audio to play for 5 seconds.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Error("Failed to sleep: ", err)
	}

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
		s.Fatal("Failed to wait for BT Headset: ", err)
	}
}
