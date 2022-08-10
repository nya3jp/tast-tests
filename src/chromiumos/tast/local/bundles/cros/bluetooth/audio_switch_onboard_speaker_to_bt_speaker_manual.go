// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AudioSwitchOnboardSpeakerToBTSpeakerManual,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies audio playback switching from onboard speaker to BT speaker",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Vars:         []string{"bluetooth.btDeviceName"},
		Fixture:      "chromeLoggedIn",
	})
}

// AudioSwitchOnboardSpeakerToBTSpeakerManual performs audio playback on internal-speaker
// before connecting bluetooth device and audio switching to bluetooth device once
// bluetooth device is connected to DUT.
// Manual step: bluetooth.btDeviceName bluetooth device has to be set to pairing mode before executing test-script.
func AudioSwitchOnboardSpeakerToBTSpeakerManual(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	btHeadset := s.RequiredVar("bluetooth.btDeviceName")
	var expectedAudioOuputNode = "INTERNAL_SPEAKER"

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	// Generate sine raw input file that lasts 60 seconds.
	rawFileName := "AudioFile.raw"
	rawFilePath := filepath.Join(downloadsPath, rawFileName)
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
		s.Fatal("Failed to generate audio test data: ", err)
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "AudioFile.wav"
	wavFile := filepath.Join(downloadsPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		s.Fatal("Failed to convert raw to wav: ", err)
	}
	defer os.Remove(wavFile)

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(ctx)

	if err := files.OpenDownloads()(ctx); err != nil {
		s.Fatal("Failed to open Downloads folder in files app: ", err)
	}
	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", wavFileName, err)
	}

	defer func() {
		// Closing the audio player.
		kb, err := input.VirtualKeyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()
		if kb.Accel(ctx, "Ctrl+W"); err != nil {
			testing.ContextLog(ctx, "Failed to close Audio player: ", err)
		}
	}()

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}

	if deviceType != expectedAudioOuputNode {
		if err := cras.SetActiveNodeByType(ctx, expectedAudioOuputNode); err != nil {
			s.Fatalf("Failed to select active device %s: %v", expectedAudioOuputNode, err)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device: ", err)
		}
		if deviceType != expectedAudioOuputNode {
			s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioOuputNode)
		}
	}

	if err := verifyAudioRoute(ctx, deviceName); err != nil {
		s.Fatalf("Failed to verify audio routing through %q: %v", expectedAudioOuputNode, err)
	}

	expectedAudioOuputNode = "BLUETOOTH"
	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth adapters: ", err)
	}

	if len(adapters) == 0 {
		s.Fatal("Failed to find bluetooth adapter")
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

	if err := bluez.DisconnectAllDevices(ctx); err != nil {
		s.Fatal("Failed to disconnect the devices: ", err)
	}

	// Waits for a specific BT device to be found.
	var btDevice *bluez.Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDevice, err = bluez.DeviceByAlias(ctx, btHeadset)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for BT Headset: ", err)
	}

	// Pair BT Device.
	if isPaired, _ := btDevice.Paired(ctx); !isPaired {
		if err := btDevice.Pair(ctx); err != nil {
			s.Fatal("Failed to pair bluetooth device: ", err)
		}
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

	// After connecting to bluetooth device audio playback must
	// switch from onboard speaker to bluetooth speaker.
	var btDeviceName, btDeviceType string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDeviceName, btDeviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get the selected audio device")
		}
		if btDeviceType != expectedAudioOuputNode {
			return errors.Wrap(err, "failed to switch to audio node BLUETOOTH")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Timeout to check for audio node switch: ", err)
	}

	if err := verifyAudioRoute(ctx, btDeviceName); err != nil {
		s.Fatalf("Failed to verify audio routing through %q: %v", expectedAudioOuputNode, err)
	}
}

// verifyAudioRoute checks whether audio is routing via deviceName or not.
func verifyAudioRoute(ctx context.Context, deviceName string) error {
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
		return errors.Wrapf(err, "timeout waiting for %q", deviceName)
	}
	return nil
}
