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

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/setup"
	deviceSpeed "chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/local/usbutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// usbFileInfo stores data common to the tests run in this package.
type usbFileInfo struct {
	// usbSpeed holds USB pendrive speed like 480M, 5000M
	usbSpeed string
	// fileSize holds file size in bytes to create a file.
	fileSize int64
	// fileName is name of the file.
	fileName string
	// deviceType is test_config key name like TBT, USB4.
	deviceType string
	// usbType is type of USB device connected like 2.0, 3.0, 3.10, 3.20.
	usbType string
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBStorageFunctionality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB pendrive functionality on TBT Dock station and USB4 Gatkex card",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json", "testcert.p12"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP"},
		Fixture:      "chromeLoggedInThunderbolt",
		HardwareDeps: hwdep.D(setup.ThunderboltSupportedDevices()),
		Params: []testing.Param{{
			Name: "usb3_pendrive_tbt_dock",
			Val: usbFileInfo{
				fileName: "usb3_test_sample_file.txt",
				// File size 2GB.
				fileSize:   1024 * 1024 * 1024 * 2,
				usbSpeed:   "5000M",
				deviceType: "TBT",
				usbType:    "3.0",
			},
			// Maximum timeout for data transfer in bidirectional and cleanup.
			Timeout: 5 * time.Minute,
		}, {
			Name: "usb2_pendrive_tbt_dock",
			Val: usbFileInfo{
				fileName: "usb2_test_sample_file.txt",
				// File size 1GB.
				fileSize:   1024 * 1024 * 1024,
				usbSpeed:   "480M",
				deviceType: "TBT",
				usbType:    "2.0",
			},
			// Maximum timeout for data transfer in bidirectional and cleanup.
			Timeout: 5 * time.Minute,
		}, {
			Name: "usb3_pendrive_usb4gatkex",
			Val: usbFileInfo{
				fileName: "usb3_usb4gatkex_test_sample_file.txt",
				// File size 2GB.
				fileSize:   1024 * 1024 * 1024 * 2,
				usbSpeed:   "5000M",
				deviceType: "USB4",
				usbType:    "3.0",
			},
			// Maximum timeout for data transfer in bidirectional and cleanup.
			Timeout: 5 * time.Minute,
		}, {
			Name: "typec_pendrive_tbt_dock",
			Val: usbFileInfo{
				fileName: "typec_test_sample_file.txt",
				// File size 10GB.
				fileSize:   1024 * 1024 * 1024 * 10,
				usbSpeed:   "5000M",
				deviceType: "TBT",
				usbType:    "3.20",
			},
			// Maximum timeout for data transfer in bidirectional and cleanup.
			Timeout: 10 * time.Minute,
		},
		}})
}

// USBStorageFunctionality performs USB pendrive functionalities on
// TBT Dock station and USB4 Gatkex card.

// Connect 3.0 USB pendrive to TBT Dock for test --> usb3_pendrive_tbt_dock.
// Connect 2.0 USB pendrive to TBT Dock for test --> usb2_pendrive_tbt_dock.
// Connect 3.0 USB pendrive to USB4 Gatkex card for test --> usb3_pendrive_usb4gatkex.
// Connect Typec USB pendrive to TBT Dock for test --> typec_pendrive_tbt_dock.

// USBStorageFunctionality requires the following H/W topology to run.
// DUT ---> C-Switch(device that performs hot plug-unplug) ---> TBT DOCK/USB4 ---> USB device.
func USBStorageFunctionality(ctx context.Context, s *testing.State) {
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

	testParms := s.Param().(usbFileInfo)

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

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
	deviceVal, ok := data[testParms.deviceType].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found TBT config data in JSON file")
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
	sourceFilePath := path.Join(sourcePath, testParms.fileName)

	// Create a file with size.
	file, err := os.Create(sourceFilePath)
	if err != nil {
		s.Fatal("Failed to create file: ", err)
	}
	if err := file.Truncate(int64(testParms.fileSize)); err != nil {
		s.Fatal("Failed to truncate file with size: ", err)
	}

	var devicePath string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		removableDevicesList, err := usbutil.RemovableDevices(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get device list")
		}
		if len(removableDevicesList.RemovableDevices) == 0 {
			return errors.New("failed to get removable devices info")
		}
		usbType := removableDevicesList.RemovableDevices[0].UsbType
		if usbType != testParms.usbType {
			return errors.Errorf("failed to get USB version type: got %q, want %q", usbType, testParms.usbType)
		}
		devicePath = removableDevicesList.RemovableDevices[0].Mountpoint
		if devicePath == "" {
			return errors.New("failed to get vaild devicePath")
		}
		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		s.Fatal("Timeout waiting for USB pendrive detection: ", err)
	}

	// Verify USB pendrive speed.
	speedOut, err := deviceSpeed.MassStorageUSBSpeed(ctx)
	if err != nil {
		s.Fatal("Failed to check for USB speed: ", err)
	}

	speedFound := false
	for _, speed := range speedOut {
		if speed == testParms.usbSpeed {
			speedFound = true
			break
		}
	}
	if !speedFound {
		s.Fatalf("Unexpected USB device speed: want %q, got %v", testParms.usbSpeed, speedOut)
	}

	// Destination file path.
	destinationFilePath := path.Join(devicePath, testParms.fileName)
	defer os.Remove(destinationFilePath)

	localHash, err := deviceSpeed.FileChecksum(sourceFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the source file: ", err)
	}

	// Tranferring file from source to destination.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	if err := deviceSpeed.CopyFile(sourceFilePath, destinationFilePath); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	destHash, err := deviceSpeed.FileChecksum(destinationFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the destination file: ", err)
	}

	if !bytes.Equal(localHash, destHash) {
		s.Errorf("The hash doesn't match (destHash path: %q)", destHash)
	}

	// Tranferring file from destination to source.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", destinationFilePath, sourceFilePath)
	if err := deviceSpeed.CopyFile(destinationFilePath, sourceFilePath); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	connected, err := cswitch.IsDeviceEnumerated(ctx, deviceVal["device_name"].(string), tbtPort)
	if connected {
		if err != nil {
			s.Fatal("Failed to disconnect the TBT device: ", err)
		}
	}
}
