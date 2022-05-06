// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
)

// usbTestParams stores data common to the tests run in this package.
type usbTestParams struct {
	// usbSpeed holds USB pendrive speed like 480M, 5000M.
	usbSpeed string
	// fileSize holds file size in bytes to create a file.
	fileSize int
	// fileName is name of the file.
	fileName string
	// deviceType is test_config key name like TBT, USB4.
	deviceType string
	// iter is iteration value for plug-unplug operation.
	iter int
	// tablet is a toggle: true for tablet mode test.
	tablet bool
	// evtestPattern is an regex string for evtest event.
	evtestPattern string
}

const (
	// oneGB is data storage capacity equivalent to one GigaByte.
	oneGB = 1024 * 1024 * 1024
	// typeAKeyboard is evtestPattern for type-A keyboard event.
	typeAKeyboard = `(?i)/dev/input/event([0-9]+):.*USB.*Keyboard.*`
	// mediaRemovable is removable media path.
	mediaRemovable = "/media/removable/"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBTypeAFunctionalityCheck,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB type-A device functionality check with consecutive hotplug-unplug using c-switch",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"c.txt", "h.txt", "r.txt", "o.txt", "m.txt", "e.txt"},
		VarDeps:      []string{"typec.cSwitchPort", "typec.domainIP"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Name: "usb2_pendrive_quick",
			Val: usbTestParams{
				fileName:   "usb_sample_file.txt",
				fileSize:   2 * oneGB,
				usbSpeed:   "480M",
				deviceType: "storage",
				iter:       1,
			},
			Timeout: 10 * time.Minute,
		}, {
			Name: "usb2_pendrive_bronze",
			Val: usbTestParams{
				fileName:   "usb_sample_file.txt",
				fileSize:   2 * oneGB,
				usbSpeed:   "480M",
				deviceType: "storage",
				iter:       10,
			},
			Timeout: 15 * time.Minute,
		}, {
			Name: "usb2_pendrive_silver",
			Val: usbTestParams{
				fileName:   "usb_sample_file.txt",
				fileSize:   2 * oneGB,
				usbSpeed:   "480M",
				deviceType: "storage",
				iter:       15,
			},
			Timeout: 20 * time.Minute,
		}, {
			Name: "usb2_pendrive_gold",
			Val: usbTestParams{
				fileName:   "usb_sample_file.txt",
				fileSize:   2 * oneGB,
				usbSpeed:   "480M",
				deviceType: "storage",
				iter:       20,
			},
			Timeout: 30 * time.Minute,
		}, {
			Name: "usb3_pendrive",
			Val: usbTestParams{
				fileName:   "usb_sample_file.txt",
				fileSize:   1 * oneGB,
				usbSpeed:   "5000M",
				deviceType: "storage",
				iter:       1,
			},
			Timeout: 10 * time.Minute,
		}, {
			Name: "typea_keyboard_quick",
			Val: usbTestParams{
				usbSpeed:      "1.5M",
				deviceType:    "Keyboard",
				iter:          1,
				evtestPattern: typeAKeyboard,
			},
		}, {
			Name: "typea_keyboard_bronze",
			Val: usbTestParams{
				usbSpeed:      "1.5M",
				deviceType:    "Keyboard",
				iter:          10,
				evtestPattern: typeAKeyboard,
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "typea_keyboard_silver",
			Val: usbTestParams{
				usbSpeed:      "1.5M",
				deviceType:    "Keyboard",
				iter:          15,
				evtestPattern: typeAKeyboard,
			},
			Timeout: 10 * time.Minute,
		}, {
			Name: "typea_keyboard_gold",
			Val: usbTestParams{
				usbSpeed:      "1.5M",
				deviceType:    "Keyboard",
				iter:          20,
				evtestPattern: typeAKeyboard,
			},
			Timeout: 15 * time.Minute,
		}, {
			Name: "typec_keyboard_quick",
			Val: usbTestParams{
				usbSpeed:      "12M",
				deviceType:    "Keyboard",
				iter:          1,
				evtestPattern: `(?i)/dev/input/event([0-9]+):.*C-Type.*`,
			},
		}, {
			Name: "typea_keyboard_tablet",
			Val: usbTestParams{
				usbSpeed:      "1.5M",
				deviceType:    "Keyboard",
				iter:          1,
				tablet:        true,
				evtestPattern: typeAKeyboard,
			},
		}},
	})
}

// USBTypeAFunctionalityCheck performs USB type-A devices functionality check
// with consecutive hotplug and unplug.
// USBTypeAFunctionalityCheck requires the following H/W topology to run.
//
// 1. DUT --> C-Switch(device that performs hot plug-unplug) --> type-C Adapter --> type-A devices.
// 2. DUT --> C-Switch(device that performs hot plug-unplug) --> type-C keyboard.
func USBTypeAFunctionalityCheck(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	testParms := s.Param().(usbTestParams)
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer cancel()

	if testParms.tablet {
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create test API connection: ", err)
		}
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
		if err != nil {
			s.Fatal("Failed to ensure in tablet mode: ", err)
		}
		defer cleanup(cleanupCtx)
	}

	// Create C-Switch session that performs hot plug-unplug on TBT/USB4 device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	const cSwitchOFF = "0"
	defer func(ctx context.Context) {
		// Remove created kbEventlog.txt file for cleanup.
		eventLogFile := "/tmp/kbEventlog.txt"
		if _, err := os.Stat(eventLogFile); err == nil {
			if err := os.Remove(eventLogFile); err != nil {
				s.Error("Failed to remove temporary log file: ", err)
			}
		}

		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Error("Failed to disable c-switch port: ", err)
		}
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Error("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)

	// Consecutive pluging and unplugging USB devices.
	iter := testParms.iter
	for i := 1; i <= iter; i++ {
		s.Logf("Hotplug - unplug iteration: %d/%d", i, iter)
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
			s.Fatal("Failed to enable c-switch port: ", err)
		}

		if testParms.deviceType == "storage" {
			if err := verifyUSBStorageDevice(ctx, testParms.usbSpeed); err != nil {
				s.Fatal("Failed to detect USB storage device: ", err)
			}
		} else if testParms.deviceType == "Keyboard" {
			if err := verifyUSBHIDDevice(ctx, testParms.usbSpeed); err != nil {
				s.Fatal("Failed to detect USB HID device: ", err)
			}

			eventNum, err := usbKeyboardEventNumber(ctx, testParms.evtestPattern)
			if err != nil {
				s.Fatal("Failed to get USB Keyboard event number in evtest: ", err)
			}

			usbKeyFiles := []string{s.DataPath("c.txt"), s.DataPath("h.txt"),
				s.DataPath("r.txt"), s.DataPath("o.txt"),
				s.DataPath("m.txt"), s.DataPath("e.txt"),
			}
			if err := performUSBKeyboardEvents(ctx, eventNum, usbKeyFiles); err != nil {
				s.Fatal("Failed to perform USB Keyboard keypress: ", err)
			}
		}

		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}

		if testParms.deviceType == "storage" {
			if err := verifyUSBStorageDevice(ctx, testParms.usbSpeed); err == nil {
				s.Fatal("USB storage device is still detecting after unplug: ", err)
			}
		} else if testParms.deviceType == "Keyboard" {
			if err := verifyUSBHIDDevice(ctx, testParms.usbSpeed); err == nil {
				s.Fatal("USB HID device is still detecting after unplug: ", err)
			}
		}
	}

	if testParms.deviceType == "storage" {
		dirsBeforePlug, err := removableDirsList()
		if err != nil {
			s.Fatal("Failed to get removable devices: ", err)
		}

		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
			s.Fatal("Failed to enable c-switch port: ", err)
		}

		// Waits for USB pendrive detection till timeout.
		var dirsAfterPlug []string
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			dirsAfterPlug, err = removableDirsList()
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

		devicePath := newMountPath(dirsAfterPlug, dirsBeforePlug)
		if devicePath == "" {
			s.Fatal("Failed to get vaild devicePath")
		}

		// Destination file path.
		destinationFilePath := path.Join(mediaRemovable, devicePath, testParms.fileName)

		defer os.Remove(destinationFilePath)

		localHash, err := fileChecksumValue(sourceFilePath)
		if err != nil {
			s.Error("Failed to calculate hash of the source file: ", err)
		}

		// Tranferring file from source to destination.
		s.Logf("Tranferring file from %s to %s", sourceFilePath, destinationFilePath)
		if err := typecutils.CopyFile(sourceFilePath, destinationFilePath); err != nil {
			s.Fatal("Failed to copy file: ", err)
		}

		destHash, err := fileChecksumValue(destinationFilePath)
		if err != nil {
			s.Error("Failed to calculate hash of the destination file: ", err)
		}

		if !bytes.Equal(localHash, destHash) {
			s.Errorf("The hash doesn't match (destHash path: %q)", destHash)
		}

		// Tranferring file from destination to source.
		s.Logf("Tranferring file from %s to %s", destinationFilePath, sourceFilePath)
		if err := typecutils.CopyFile(destinationFilePath, sourceFilePath); err != nil {
			s.Fatal("Failed to copy file: ", err)
		}
	}
}

// fileChecksumValue checks the checksum for the input file.
func fileChecksumValue(path string) ([]byte, error) {
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

// removableDirsList lists the connected removable devices.
func removableDirsList() ([]string, error) {
	fis, err := ioutil.ReadDir(mediaRemovable)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read directory")
	}
	var ret []string
	for _, fi := range fis {
		ret = append(ret, fi.Name())
	}
	return ret, nil
}

// newMountPath returns the first new item in dirsAfterPlug not in dirsBeforePlug.
func newMountPath(dirsAfterPlug, dirsBeforePlug []string) string {
	for _, afterPlug := range dirsAfterPlug {
		found := false
		for _, beforePlug := range dirsBeforePlug {
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

// verifyUSBStorageDevice returns USB pendrive detection status.
func verifyUSBStorageDevice(ctx context.Context, usbSpeed string) error {
	speedFound := false
	return testing.Poll(ctx, func(ctx context.Context) error {
		speedOut, err := typecutils.MassStorageUSBSpeed(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get USB speed")
		}
		for _, speed := range speedOut {
			if speed == usbSpeed {
				speedFound = true
				break
			}
		}
		if !speedFound {
			return errors.Errorf("unexpected USB HID speed = got %q, want %q", speedOut, usbSpeed)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// usbHIDSpeed returns Human Interface Device(HID) speed for HID-USB devices.
// It returns error with empty speed slice when it encounters an error.
func usbHIDSpeed(ctx context.Context) ([]string, error) {
	res, err := typecutils.ListDevicesInfo(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get lsusb details")
	}

	var speedSlice []string
	for _, dev := range res {
		if dev.Class == "Human Interface Device" {
			devSpeed := dev.Speed
			if devSpeed != "" {
				speedSlice = append(speedSlice, devSpeed)
			}
		}
	}
	if len(speedSlice) == 0 {
		return nil, errors.New("failed to find USB HID device speed")
	}
	return speedSlice, nil
}

// verifyUSBHIDDevice only returns error when it fails to find a HID device
// with expected USB speed. Otherwise, it returns nil.
func verifyUSBHIDDevice(ctx context.Context, usbSpeed string) error {
	speedFound := false
	return testing.Poll(ctx, func(ctx context.Context) error {
		speedOut, err := usbHIDSpeed(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get USB speed")
		}
		for _, speed := range speedOut {
			if speed == usbSpeed {
				speedFound = true
				break
			}
		}
		if !speedFound {
			return errors.Errorf("unexpected USB HID speed = got %q, want %q", speedOut, usbSpeed)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}

// usbKeyboardEventNumber returns USB Keyboard evtest event number.
func usbKeyboardEventNumber(ctx context.Context, evtestPattern string) (int, error) {
	out, _ := exec.Command("evtest").CombinedOutput()
	re := regexp.MustCompile(evtestPattern)
	result := re.FindStringSubmatch(string(out))
	keyboardEventNum := ""
	if len(result) > 0 {
		keyboardEventNum = result[1]
	} else {
		return 0, errors.New("Keyboard has not found in evtest command output")
	}
	eventNum, err := strconv.Atoi(keyboardEventNum)
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	return eventNum, nil
}

// performUSBKeyboardEvents performs USB Keyboard key press events.
func performUSBKeyboardEvents(ctx context.Context, eventNum int, usbKeyFiles []string) error {
	eventLogFile := "/tmp/kbEventlog.txt"
	evtestRecordCmd := "evtest /dev/input/event"

	// Perform evtest command to record all events and save in temporary file.
	var err error
	go func() {
		err = testexec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("%s%d > %s &", evtestRecordCmd, eventNum, eventLogFile)).Run()
	}()
	if err != nil {
		return errors.Wrap(err, "failed to perform evtest events record")
	}

	// Perform USB Key press.
	usbKeyPlay := "evemu-play --insert-slot0 /dev/input/event"
	for _, keyFile := range usbKeyFiles {
		if err := testexec.CommandContext(ctx, "bash", "-c", usbKeyPlay+strconv.Itoa(eventNum)+" < "+keyFile).Run(); err != nil {
			return errors.Wrap(err, "failed to play USB Keyboard event")
		}
	}

	// Stopping evtest record process.
	if err := testexec.CommandContext(ctx, "sudo", "pkill", "evtest").Run(); err != nil {
		return errors.Wrap(err, "failed to kill evtest process")
	}

	catOutput, err := testexec.CommandContext(ctx, "cat", eventLogFile).Output()
	if err != nil {
		return errors.Wrap(err, "failed to execute cat command")
	}

	// Validating USB key press is recorded in log file output.
	keysPattern := []string{"KEY_C", "KEY_H", "KEY_R", "KEY_O", "KEY_M", "KEY_E"}
	for _, key := range keysPattern {
		keyRe := regexp.MustCompile(fmt.Sprintf(`\(%s\).*value 0`, key))
		match := keyRe.FindAllString(string(catOutput), -1)
		if len(match) == 0 {
			return errors.Errorf("failed to press USB Keyboard %q key", key)
		}
	}
	return nil
}
