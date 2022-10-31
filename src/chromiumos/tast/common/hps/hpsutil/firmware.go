// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hpsutil contains functionality used by the HPS tast tests.
package hpsutil

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

const (
	// P2PowerCycleFilename is the name of the script that power-cycles the p2 board.
	P2PowerCycleFilename = "proto2-power-cycle.py"

	// devboardDevice is the argument passed to the hps tool for SantaP2.
	devboardDevice = "--mcp"
	// builtinDevice is the argument passed to the hps tool for the builtin HPS module.
	// builtinDevice = "--bus=/dev/i2c-15"
	builtinDevice = "--bus=/dev/i2c-hps-controller"

	usbhidPath = "/sys/bus/usb/drivers/usbhid"

	// Stage1 verification done by Stage0 takes approximately 700ms.
	// We want to use slightly longer pause to make this non-flaky.
	firmwareVerificationDelay = time.Second * 2

	// FirmwarePath is the path on the DUT filesystem where HPS firmware images
	// are stored.
	FirmwarePath = "/usr/lib/firmware/hps/"
	// Paths to different firmware blobs.
	stage1Name          = "mcu_stage1.bin"
	versionFileName     = "mcu_stage1.version.txt"
	fpgaBitstreamName   = "fpga_bitstream.bin"
	fpgaApplicationName = "fpga_application.bin"

	// LatestFirmwarePath is the path on the DUT where "latest" firmware images
	// are stored. This firmware was built from source during the build
	// process. The images are not signed and represent the latest unreleased
	// version of firmware, unlike the released firmware in /usr/lib/firmware/hps.
	LatestFirmwarePath = "/usr/lib/firmware/hps/latest"
)

// DeviceType specifies which device to flash.
type DeviceType int

const (
	// DeviceTypeDevboard is Santa p2 board plugged using USB.
	DeviceTypeDevboard = iota

	// DeviceTypeBuiltin is the built-in HPS connected over i2c bus.
	DeviceTypeBuiltin
)

func unbindUsbHid(hctx *HpsContext) error {
	if hctx.DutConn != nil {
		return remoteUnbindUsbHid(hctx)
	}
	return localUnbindUsbHid(hctx.Ctx)
}

func remoteUnbindUsbHid(hctx *HpsContext) error {
	dconn := hctx.DutConn
	ctx := hctx.Ctx
	list, err := dconn.CommandContext(ctx, "ls", usbhidPath).Output()
	if err != nil {
		return err
	}
	files := strings.Split(string(list), "\n")

	usbHidRegex := regexp.MustCompile(`^\d+`)
	for _, file := range files {
		matched := usbHidRegex.MatchString(file)
		if matched {
			testing.ContextLog(ctx, "usbhid: Unbinding ", file)
			_, err := dconn.CommandContext(hctx.Ctx, "echo", file, ">>",
				path.Join(usbhidPath, "unbind")).Output()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func localUnbindUsbHid(ctx context.Context) error {
	// Unbind the USB to stop getting USB `claim interface: Resource busy` errors.
	// This needs to be done once after SantaP2 is re-plugged.
	files, err := ioutil.ReadDir(usbhidPath)
	if err != nil {
		return errors.Wrap(err, "unable to list files in usbhid")
	}

	usbHidRegex := regexp.MustCompile(`^\d+`)

	for _, file := range files {
		matched := usbHidRegex.MatchString(file.Name())
		if matched {
			testing.ContextLog(ctx, "usbhid: Unbinding ", file.Name())
			err := ioutil.WriteFile(path.Join(usbhidPath, "unbind"), []byte(file.Name()), 0300)
			if err != nil {
				return errors.Wrapf(err, "unable to unbind %s", file.Name())
			}
		}
	}

	return nil
}

func flashWithRetries(hctx *HpsContext, powerCycleFunc func(*HpsContext) error, cmd []string) error {
	var err error
	if err = RunHpsTool(hctx, cmd...); err != nil {
		for i := 1; i < 3; i++ {
			// Rarely it doesn't work on the first try, retry after delay.
			testing.ContextLog(hctx.Ctx, "Failed. Retry ", i)
			if err = powerCycleFunc(hctx); err != nil {
				return err
			}
			if err = testing.Sleep(hctx.Ctx, time.Second*3); err != nil {
				return errors.Wrap(err, "sleep failed")
			}
			if err = RunHpsTool(hctx, cmd...); err != nil {
				RunStatus(hctx)
				continue
			}
			break
		}
	}
	RunStatus(hctx)
	return err
}

// EnsureLatestFirmware flashes all firmware to the HPS board.
//
// Prerequisites for Santa:
// 1. ./scripts/p2-stage0-run
// 2. cd mcu_rom/hps-factory && cargo run -- --mcp --one-time-init
func EnsureLatestFirmware(hctx *HpsContext) error {
	testing.ContextLog(hctx.Ctx, "Started HPS firmware update")

	if err := hctx.PowerCycle(); err != nil {
		return err
	}

	// stage0: Expect it to be initialized by hpsd automatically.
	// Nothing should break even if we don't update it.
	//
	// In order to update it manually run ./scripts/p2-stage0-run in hps-firmware on gLinux box.

	// Flash stage1.
	// http://b/191716856: Read (retries exceeded) failed
	powerCycleFunc := func(hctx *HpsContext) error {
		return hctx.PowerCycle()
	}
	binFilepaths, err := decompressBin(hctx.Ctx)
	if err != nil {
		return errors.Wrap(err, "Decompress bin files failed")
	}
	stage1Path := binFilepaths[stage1Name]
	fpgaBitstreamPath := binFilepaths[fpgaBitstreamName]
	fpgaApplicationPath := binFilepaths[fpgaApplicationName]
	tmpDir := filepath.Dir(stage1Path)
	defer os.RemoveAll(tmpDir)

	if err := flashWithRetries(hctx, powerCycleFunc, []string{"dl", "0", stage1Path}); err != nil {
		return errors.Wrap(err, "Need to update stage0 or replug the device?")
	}

	// Reset stage0.
	if err := RunHpsTool(hctx, "cmd", "reset"); err != nil {
		return errors.Wrap(err, "Run this command in hps-firmware to fix dev board: cd hps-firmware2/mcu_rom/hps-factory && cargo run -- --mcp --one-time-init")
	}

	if err := testing.Sleep(hctx.Ctx, firmwareVerificationDelay); err != nil {
		return errors.Wrap(err, "sleep failed")
	}

	if err := RunStatus(hctx); err != nil {
		return err
	}

	// Need to start hps in order to be able to do `dl 1`.
	powerCyclePlusCmdLaunchFunc := func(hctx *HpsContext) error {
		if err := hctx.PowerCycle(); err != nil {
			return err
		}
		if err := RunHpsTool(hctx, "cmd", "launch"); err != nil {
			return err
		}
		return nil
	}

	// Need to call `cmd launch` before flashing.
	if err := powerCyclePlusCmdLaunchFunc(hctx); err != nil {
		return err
	}

	if err := flashWithRetries(hctx, powerCyclePlusCmdLaunchFunc, []string{"dl", "1", fpgaBitstreamPath}); err != nil {
		return err
	}

	// b/209533097: dml@ recommends to add a power-cycle here to work-around the SPI data corruption.
	if err := powerCyclePlusCmdLaunchFunc(hctx); err != nil {
		return err
	}

	if err := flashWithRetries(hctx, powerCyclePlusCmdLaunchFunc, []string{"dl", "2", fpgaApplicationPath}); err != nil {
		return err
	}

	testing.ContextLog(hctx.Ctx, "Finished HPS firmware update")
	return nil
}

func decompressBin(ctx context.Context) (map[string]string, error) {
	tmpDir, err := ioutil.TempDir("", "hps_firmware")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test directory under /tmp")
	}
	testing.ContextLog(ctx, "tmp dir: ", tmpDir)

	firmwareTmpPath := strings.TrimSpace(string(tmpDir))

	files, err := ioutil.ReadDir(FirmwarePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list files under firmware dir")
	}

	paths := make(map[string]string)
	for _, file := range files {
		if !strings.Contains(file.Name(), ".xz") {
			continue
		}

		originPath := filepath.Join(FirmwarePath, file.Name())
		filePrefix := strings.Replace(file.Name(), ".xz", "", -1)

		if _, err := testexec.CommandContext(ctx, "cp", originPath, firmwareTmpPath).Output(); err != nil {
			return nil, errors.Wrap(err, "copying file failed")
		}
		if _, err := testexec.CommandContext(ctx, "unxz", filepath.Join(firmwareTmpPath, file.Name())).Output(); err != nil {
			return nil, errors.Wrap(err, "failed to unzip the xz file")
		}
		paths[filePrefix] = filepath.Join(firmwareTmpPath, filePrefix)
	}
	return paths, nil
}

// FetchRunningFirmwareVersion reads out the version of the firmware currently running on HPS.
//
// This assumes HPS is powered on and running in the application
// (that is, hpsd has finished enabling features).
func FetchRunningFirmwareVersion(hctx *HpsContext) (int32, error) {
	testing.ContextLog(hctx.Ctx, "Checking running firmware version reported by HPS")

	versionHigh, err := GetRegisterValue(hctx, "10")
	if err != nil {
		return 0, errors.Wrap(err, "failed to read firmware version high byte register")
	}

	versionLow, err := GetRegisterValue(hctx, "11")
	if err != nil {
		return 0, errors.Wrap(err, "failed to read firmware version low byte register")
	}

	version := int32(versionHigh)<<16 | int32(versionLow)
	testing.ContextLog(hctx.Ctx, "HPS running version: ", version)
	return version, nil
}

// FetchFirmwareVersionFromImage determines the version of the firmware stored in
// the ChromeOS image running on the DUT.
func FetchFirmwareVersionFromImage(hctx *HpsContext, firmwarePath string) (int32, error) {
	firmwareVersionFilePath := filepath.Join(firmwarePath, versionFileName)

	var versionBytes []byte
	var err error
	if hctx.DutConn != nil {
		versionBytes, err = linuxssh.ReadFile(hctx.Ctx, hctx.DutConn, firmwareVersionFilePath)
	} else {
		versionBytes, err = ioutil.ReadFile(firmwareVersionFilePath)
	}
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read firmware version from %v", firmwareVersionFilePath)
	}

	version, err := strconv.ParseInt(strings.TrimSpace(string(versionBytes)), 10, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to decode firmware version from %v", firmwareVersionFilePath)
	}

	testing.ContextLogf(hctx.Ctx, "Found firmware version from %v: %v", firmwareVersionFilePath, version)
	return int32(version), nil
}
