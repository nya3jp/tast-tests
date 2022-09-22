// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"time"

	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bundles/cros/typec/setup"
	deviceSpeed "chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/youtube"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/local/usbutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ThunderboltDockStationFunctionality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies Thunderbolt dock station functionality with various devices connected",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json", "testcert.p12"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		Fixture:      "chromeLoggedInThunderbolt",
		HardwareDeps: hwdep.D(setup.ThunderboltSupportedDevices()),
		Timeout:      5 * time.Minute,
	})
}

// ThunderboltDockStationFunctionality performs connected devices functionality check on
// Thunderbolt(TBT) Dock station.

// Connect USB3.0 pendrive, Type-C pendrive, USB Keyboard, USB Mouse, 3.5mm headset
// to TBT Dock for this test.

// ThunderboltDockStationFunctionality requires the following H/W topology to run.
// DUT ---> C-Switch(device that performs hot plug-unplug) --
// ---> TBT DOCK ---> USB3.0 pendrive, Type-C pendrive, USB Keyboard, USB Mouse, 3.5mm Jack Headset.
func ThunderboltDockStationFunctionality(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	// Config file which contains expected values of USB4/TBT parameters.
	const testConfig = "test_config.json"
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	vkb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard eventwriter: ", err)
	}

	// Mute the device to avoid noisiness.
	if err := crastestclient.Mute(ctx); err != nil {
		s.Fatal("Failed to mute: ", err)
	}
	defer crastestclient.Unmute(cleanupCtx)

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatal("Failed to read response data: ", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for TBT config data.
	deviceVal, ok := data["TBT"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to find TBT config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on TBT/USB4 device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	const cSwitchOFF = "0"
	defer func(ctx context.Context) {
		s.Log("Cleanup")
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, deviceVal["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to enumerate the TBT device: ", err)
	}

	sourcePath, err := ioutil.TempDir("", "temp")
	if err != nil {
		s.Fatal("Failed to create temp directory: ", err)
	}
	defer os.RemoveAll(sourcePath)

	// Source file path.
	const fileName = "usb_sample_file.txt"
	sourceFilePath := path.Join(sourcePath, fileName)

	// Create a file with size.
	file, err := os.Create(sourceFilePath)
	if err != nil {
		s.Fatal("Failed to create file: ", err)
	}

	// fileSize of 2GB.
	const fileSize = 1024 * 1024 * 1024 * 2
	if err := file.Truncate(int64(fileSize)); err != nil {
		s.Fatal("Failed to truncate file with size: ", err)
	}

	// Verify USB pendrive speed.
	usbDeviceClassName := "Mass Storage"
	usbSpeed := "5000M"
	const noOfConnectedDevice = 2
	var usbDevicesList []usbutils.USBDevice
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		usbDevicesList, err = usbutils.ListDevicesInfo(ctx, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get USB devices list")
		}
		got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbDeviceClassName, usbSpeed)
		if want := noOfConnectedDevice; got != want {
			return errors.Errorf("unexpected number of USB storage devices connected with %q speed:	got %d, want %d",
				usbSpeed, got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Timeout waiting for USB pendrive speed detection: ", err)
	}

	usbType := "3.0"
	usb3DevicePath, err := usbutil.USBStorageDevicePath(ctx, usbType)
	if err != nil {
		s.Fatalf("Failed to get %q USB type device path: %v", usbType, err)
	}

	usbType = "3.20"
	usbTypecDevicePath, err := usbutil.USBStorageDevicePath(ctx, usbType)
	if err != nil {
		s.Fatalf("Failed to get %q USB type device path: %v", usbType, err)
	}

	usbDeviceClassName = "Human Interface Device"
	usbSpeed = "1.5M"
	got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbDeviceClassName, usbSpeed)
	if want := noOfConnectedDevice; got != want {
		s.Fatalf("Unexpected number of USB HID devices connected with %q speed:	got %d, want %d",
			usbSpeed, got, want)
	}

	// Destination file path.
	destinationFilePath := path.Join(usb3DevicePath, fileName)
	defer os.Remove(destinationFilePath)

	if err := performBiDirectionFileTransfer(ctx, sourceFilePath, destinationFilePath); err != nil {
		s.Fatal("Failed to transfer file from DUT to USB3.0 pendrive and vice-versa: ", err)
	}

	destinationFilePath = path.Join(usbTypecDevicePath, fileName)
	defer os.Remove(destinationFilePath)

	if err := performBiDirectionFileTransfer(ctx, sourceFilePath, destinationFilePath); err != nil {
		s.Fatal("Failed to transfer file from DUT to USB typec pendrive and vice-versa: ", err)
	}

	var videoSource = youtube.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "1440p60",
	}

	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	cui := uiauto.New(tconn)
	isExternalDisplay := false
	videoApp := youtube.NewYtWeb(cr.Browser(), tconn, vkb, videoSource, isExternalDisplay, cui, uiHandler)
	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	if err := checkAudioPlay(ctx); err != nil {
		s.Fatal("Failed to check audio play via 3.5mm Headset: ", err)
	}
}

// performBiDirectionFileTransfer perform file transfer from source to destination and vice-versa.
func performBiDirectionFileTransfer(ctx context.Context, sourceFilePath, destinationFilePath string) error {
	localHash, err := deviceSpeed.FileChecksum(sourceFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to calculate hash of the source file")
	}

	// Tranferring file from source to destination.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	if err := deviceSpeed.CopyFile(sourceFilePath, destinationFilePath); err != nil {
		return errors.Wrapf(err, "failed to copy file from %s to %s", sourceFilePath, destinationFilePath)
	}

	destHash, err := deviceSpeed.FileChecksum(destinationFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to calculate hash of the destination file")
	}

	if !bytes.Equal(localHash, destHash) {
		return errors.Errorf("the hash doesn't match (destHash path: %q)", destHash)
	}

	// Tranferring file from destination to source.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", destinationFilePath, sourceFilePath)
	if err := deviceSpeed.CopyFile(destinationFilePath, sourceFilePath); err != nil {
		return errors.Wrapf(err, "failed to copy file from %s to %s", destinationFilePath, sourceFilePath)
	}

	localHash, err = deviceSpeed.FileChecksum(sourceFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to calculate hash of the source file")
	}

	if !bytes.Equal(localHash, destHash) {
		return errors.Errorf("the hash doesn't match (localHash path: %q)", localHash)
	}

	return nil
}

// checkAudioPlay sets and verifies audio is routing through 3.5mm Headset.
func checkAudioPlay(ctx context.Context) error {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create Cras object")
	}

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get the selected audio device")
	}

	// In Thunderbolt Docking station, 3.5mm Headset will detect as USB audio device.
	var expectedAudioNode = "USB"
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

	// Verify whether audio is routing through 3.5mm Headset or not.
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
		return errors.Wrap(err, "timeout waiting for 3.5mm Headset")
	}
	return nil
}
