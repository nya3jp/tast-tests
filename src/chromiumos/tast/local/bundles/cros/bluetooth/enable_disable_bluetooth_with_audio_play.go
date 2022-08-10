// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnableDisableBluetoothWithAudioPlay,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies audio playback over BT speaker while performing bluetooth enable and disable",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"bluetooth.btDeviceName"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name:    "bronze",
			Val:     10,
			Timeout: 10 * time.Minute,
		}, {
			Name:    "silver",
			Val:     15,
			Timeout: 15 * time.Minute,
		}, {
			Name:    "gold",
			Val:     20,
			Timeout: 20 * time.Minute,
		}},
	})
}

func EnableDisableBluetoothWithAudioPlay(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	btHeadset := s.RequiredVar("bluetooth.btDeviceName")

	adapters, err := bluez.Adapters(ctx)
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
		defer adapter.SetPowered(cleanupCtx, false)
	}

	if err := adapter.StartDiscovery(ctx); err != nil {
		s.Fatal("Failed to enable discovery: ", err)
	}

	// Waits for a specific BT device to be found.
	var btDevice *bluez.Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDevice, err = bluez.DeviceByAlias(ctx, btHeadset)
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
		defer btDevice.Disconnect(cleanupCtx)
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}
	iter := s.Param().(int)
	duration := iter * 60
	devName, err := audioPlayback(ctx, tconn, downloadsPath, duration)
	if err != nil {
		s.Fatal("Failed to play audio: ", err)
	}

	ui := uiauto.New(tconn)
	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
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
		// Remove audio file at cleanup.
		audioFilePath := filepath.Join(downloadsPath, "AudioFile.wav")
		if _, err := os.Stat(audioFilePath); err == nil {
			if err := os.Chmod(audioFilePath, 777); err == nil {
				if err := os.Remove(audioFilePath); err != nil {
					s.Error("Failed to remove audio file at cleanup: ", err)
				}
			}
		}
	}(cleanupCtx)

	// Bluetooth button in the quick setting menu, when Bluetooth is on.
	bluetoothTurnOffButton := nodewith.NameContaining("Toggle Bluetooth. Connected to " + btHeadset).Role(role.ToggleButton)
	// Bluetooth button in the quick setting menu, when Bluetooth is off.
	bluetoothTurnOnButton := nodewith.NameContaining("Toggle Bluetooth. Bluetooth is off").Role(role.ToggleButton)
	// Bluetooth device button when bluetooth is on.
	btDeviceNode := nodewith.Name(btHeadset + ", Audio device").Role(role.Button)

	for i := 1; i <= iter; i++ {
		testing.ContextLogf(ctx, "Iteration %d/%d", i, iter)

		if err := quicksettings.Expand(ctx, tconn); err != nil {
			s.Fatal("Failed to open and expand the Quick Settings: ", err)
		}

		// Click on Bluetooth UI button and wait for button state to toggle.
		testing.ContextLog(ctx, "Toggling off bluetooth")
		if err := uiauto.Combine("disable Bluetooth and confirm",
			ui.LeftClick(bluetoothTurnOffButton),
			// Confirm Bluetooth adapter is disabled.
			bluez.PollForBTDisabled,
		)(ctx); err != nil {
			s.Fatal("Failed to disable Bluetooth via toggle button: ", err)
		}

		if err := verifyRunningDevice(ctx, devName); err == nil {
			s.Fatal("Failed: bluetooth disabled in DUT but audio is still running via ", btHeadset)
		}

		// Click on Bluetooth UI button and wait for button state to toggle.
		testing.ContextLog(ctx, "Toggling on bluetooth")
		if err := uiauto.Combine("enable Bluetooth and confirm",
			ui.LeftClick(bluetoothTurnOnButton),
			// Confirm Bluetooth adapter is enabled.
			bluez.PollForBTEnabled,
			// Wait for bluetooth device in scanned device list.
			ui.WaitForLocation(btDeviceNode),
			// Left click on bluetooth device.
			ui.LeftClick(btDeviceNode),
		)(ctx); err != nil {
			s.Fatal("Failed to enable Bluetooth via toggle button: ", err)
		}

		// After bluetooth on/off, audio file playback may pause,
		// play audio file if paused and check audio route on bluetooth device.
		if err := verifyRunningDevice(ctx, devName); err != nil {
			playPauseButton := nodewith.Name("Toggle play pause").Role(role.Button)
			if err := ui.LeftClick(playPauseButton)(ctx); err != nil {
				s.Fatal("Failed to select btDeviceNode: ", err)
			}
		}

		if err := verifyRunningDevice(ctx, devName); err != nil {
			s.Fatal("Failed to verify audio routing: ", err)
		}
	}
}

// audioPlayback generates wav audio file, play using default player
// and verify audio routing through bluetooth device.
func audioPlayback(ctx context.Context, tconn *chrome.TestConn, downloadsPath string, duration int) (string, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create Cras object")
	}

	var expectedAudioNode = "BLUETOOTH"

	var deviceName, deviceType string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Get current audio output device info.
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
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
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return "", err
	}

	// Generate sine raw input file that lasts for provided duration.
	rawFileName := "AudioFile.raw"
	rawFilePath := filepath.Join(downloadsPath, rawFileName)
	rawFile := audio.TestRawData{
		Path:          rawFilePath,
		BitsPerSample: 16,
		Channels:      2,
		Rate:          48000,
		Frequencies:   []int{440, 440},
		Volume:        0.05,
		Duration:      duration,
	}
	if err := audio.GenerateTestRawData(ctx, rawFile); err != nil {
		return "", errors.Wrap(err, "failed to generate audio test data")
	}
	defer os.Remove(rawFile.Path)

	wavFileName := "AudioFile.wav"
	wavFile := filepath.Join(downloadsPath, wavFileName)
	if err := audio.ConvertRawToWav(ctx, rawFilePath, wavFile, 48000, 2); err != nil {
		return "", errors.Wrap(err, "failed to convert raw to wav")
	}

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return "", errors.Wrap(err, "failed to launch the Files App")
	}
	defer files.Close(ctx)

	if err := files.OpenDownloads()(ctx); err != nil {
		return "", errors.Wrap(err, "failed to open Downloads folder in files app")
	}
	if err := files.OpenFile(wavFileName)(ctx); err != nil {
		return "", errors.Wrapf(err, "failed to open the audio file %q", wavFileName)
	}

	// Verify whether audio is routing through BT device or not.
	if err := verifyRunningDevice(ctx, deviceName); err != nil {
		return "", errors.Wrap(err, "failed to get audio running device")
	}
	return deviceName, nil
}

// verifyRunningDevice checks whether audio is routing via deviceName or not.
func verifyRunningDevice(ctx context.Context, deviceName string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
