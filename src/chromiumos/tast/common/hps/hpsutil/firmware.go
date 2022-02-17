// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hpsutil contains functionality used by the HPS tast tests.
package hpsutil

import (
	"context"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
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

	// Paths to different firmware blobs.
	stage0Path          = "/usr/lib/firmware/hps/mcu_stage0.bin"
	stage1Path          = "/usr/lib/firmware/hps/mcu_stage1.bin"
	fpgaBitstreamPath   = "/usr/lib/firmware/hps/fpga_bitstream.bin"
	fpgaApplicationPath = "/usr/lib/firmware/hps/fpga_application.bin"
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
