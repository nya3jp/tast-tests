// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
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
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// usbFileInfo stores data common to the tests run in this package.
type usbFileInfo struct {
	// usbSpeed holds USB pendrive speed like 480M, 5000M
	usbSpeed string
	// fileSize holds file size in bytes to create a file.
	fileSize int
	// fileName is name of the file.
	fileName string
	// deviceType is test_config key name like TBT, USB4.
	deviceType string
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
			},
			// Maximum timeout for data transfer in bidirectional and cleanup.
			Timeout: 5 * time.Minute,
		},
		}})
}

// USBStorageFunctionality performs USB pendrive functionalities on
// TBT Dock station and USB4 Gatkex card.

// Connect 3.0 USB pendrive to TBT Dock for test --> usb3_pendrive_tbt_dock.
// Connect 2.0 USB pendrive to TBT Dock for test --> usb2_pendrive_tbt_dock.
// Connect 3.0 USB pendrive to USB4 Gatkex card for test --> usb3_pendrive_usb4gatkex.

// USBStorageFunctionality requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->TBT DOCK and USB4.
func USBStorageFunctionality(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	// Config file which contains expected values of USB4/TBT parameters.
	const testConfig = "test_config.json"
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	// Media removable path.
	const mediaRemovable = "/media/removable/"

	testParms := s.Param().(usbFileInfo)
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	dirsBeforePlug, err := removableDirs(mediaRemovable)
	if err != nil {
		s.Fatal("Failed to get removable devices: ", err)
	}

	// Create C-Switch session that performs hot plug-unplug on TBT/USB4 device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	const cSwitchOFF = "0"
	defer func() {
		s.Log("Cleanup")
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}
	}()

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

	var dirsAfterPlug []string
	// Waits for USB pendrive detection till timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dirsAfterPlug, err = removableDirs(mediaRemovable)
		if err != nil {
			return errors.Wrap(err, "failed to get removable devices")
		}
		if len(dirsBeforePlug) >= len(dirsAfterPlug) {
			return errors.New("failed to mount removable devices")
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
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

	devicePath := tbtMountPath(dirsAfterPlug, dirsBeforePlug)
	if devicePath == "" {
		s.Fatal("Failed to get vaild devicePath")
	}

	// Destination file path.
	destinationFilePath := path.Join(mediaRemovable, devicePath, testParms.fileName)

	defer os.Remove(destinationFilePath)

	localHash, err := fileChecksum(sourceFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the source file: ", err)
	}

	// Tranferring file from source to destination.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	if err := deviceSpeed.CopyFile(sourceFilePath, destinationFilePath); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	destHash, err := fileChecksum(destinationFilePath)
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

// fileChecksum checks the checksum for the input file.
func fileChecksum(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to open files")
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return []byte{}, errors.Wrap(err, "failed to calculate the hash of the files")
	}

	return h.Sum(nil), nil
}

// removableDirs returns the connected removable devices.
func removableDirs(mountPath string) ([]string, error) {
	fis, err := ioutil.ReadDir(mountPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read directory")
	}
	var ret []string
	for _, fi := range fis {
		ret = append(ret, fi.Name())
	}
	return ret, nil
}

// tbtMountPath returns the latest removable device.
func tbtMountPath(dirsAfterPlug, dirsbeforePlug []string) string {
	for _, afterPlug := range dirsAfterPlug {
		found := false
		for _, beforePlug := range dirsbeforePlug {
			if afterPlug == beforePlug {
				found = true
				break
			}
		}
		if !found {
			return afterPlug
		}
	}
	return ""
}
