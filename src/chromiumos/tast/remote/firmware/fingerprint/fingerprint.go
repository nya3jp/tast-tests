// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's fingerprint MCU.

package fingerprint

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	// nocturne and nami are special cases and have "_fp" appended.
	// Newer FPMCUs have unique names.
	// See go/cros-fingerprint-firmware-branching-and-signing.
	fingerprintBoardNameSuffix  = "_fp"
	fingerprintFirmwarePathBase = "/opt/google/biod/fw/"
	// WaitForBiodToStartTimeout is the time to wait for biod to start.
	WaitForBiodToStartTimeout = 30 * time.Second
)

func boardFromCrosConfig(ctx context.Context, d *dut.DUT) (string, error) {
	out, err := d.Command("cros_config", "/fingerprint", "board").Output(ctx)
	return string(out), err
}

// Board returns the name of the fingerprint EC on the DUT
func Board(ctx context.Context, d *dut.DUT) (string, error) {
	// For devices that don't have unibuild support (which is required to
	// use cros_config).
	// TODO(https://crbug.com/1030862): remove when nocturne has cros_config
	// support.
	board, err := reporters.New(d).Board(ctx)
	if err != nil {
		return "", err
	}
	if board == "nocturne" {
		return board + fingerprintBoardNameSuffix, nil
	}

	// Use cros_config to get fingerprint board.
	return boardFromCrosConfig(ctx, d)
}

func firmwarePath(ctx context.Context, d *dut.DUT, fpBoard string) (string, error) {
	cmd := fmt.Sprintf("ls %s%s*.bin", fingerprintFirmwarePathBase, fpBoard)
	out, err := d.Command("bash", "-c", cmd).Output(ctx)
	if err != nil {
		return "", err
	}
	outStr := strings.TrimSpace(string(out))
	if strings.Contains(outStr, "\n") {
		return "", errors.Errorf("found multiple firmware files for %q: %s", fpBoard, outStr)
	}
	return outStr, nil
}

// FlashFirmware flashes the original fingerprint firmware in rootfs.
func FlashFirmware(ctx context.Context, d *dut.DUT) error {
	fpBoard, err := Board(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get fp board")
	}
	testing.ContextLogf(ctx, "fp board name: %q", fpBoard)

	fpFirmwarePath, err := firmwarePath(ctx, d, fpBoard)
	if err != nil {
		return errors.Wrap(err, "failed to get fp firmware path")
	}
	flashCmd := []string{"flash_fp_mcu", fpFirmwarePath}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(flashCmd))
	if err := d.Command(flashCmd[0], flashCmd[1:]...).Run(ctx); err != nil {
		return errors.Wrap(err, "flash_fp_mcu failed")
	}
	testing.ContextLog(ctx, "Flashed FP firmware, now initializing the entropy")
	if err := d.Command("bio_wash", "--factory_init").Run(ctx); err != nil {
		return errors.Wrap(err, "failed to initialize entropy after flashing FPMCU")
	}
	testing.ContextLog(ctx, "Entropy initialized, now rebooting to get seed")
	if err := d.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot DUT")
	}
	return nil
}

// InitializeKnownState checks that the AP can talk to FPMCU. If not, it flashes the FPMCU.
func InitializeKnownState(ctx context.Context, d *dut.DUT, outdir string) error {
	// Check version and see if the FPMCU is in a good state.
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(versionCmd))
	out, err := d.Command(versionCmd[0], versionCmd[1:]...).Output(ctx)

	if err == nil {
		versionOutputFile := "cros_fp_version.txt"
		testing.ContextLogf(ctx, "Writing FP firmware version to %s", versionOutputFile)
		if err := ioutil.WriteFile(filepath.Join(outdir, versionOutputFile), out, 0644); err != nil {
			// This is a nonfatal error that shouldn't kill the test.
			testing.ContextLog(ctx, "Failed to write FP firmware version to file: ", err)
		}
	} else {
		testing.ContextLogf(ctx, "Failed to query FPMCU version (error: %v). Trying re-flashing FP firmware", err)
		if err := FlashFirmware(ctx, d); err != nil {
			return errors.Wrap(err, "failed to flash FP firmware")
		}
	}
	return nil
}

// InitializeHWAndSWWriteProtect ensures hardware and software write protect are initialized as requested.
func InitializeHWAndSWWriteProtect(ctx context.Context, d *dut.DUT, pxy *servo.Proxy, enableHWWP, enableSWWP bool) error {
	testing.ContextLogf(ctx, "Initializing HW WP to %t, SW WP to %t", enableHWWP, enableSWWP)
	// HW write protect must be disabled to disable SW write protect.
	hwWPArg := "force_off"
	if !enableSWWP {
		if err := pxy.Servo().SetStringAndCheck(ctx, servo.FWWPState, hwWPArg); err != nil {
			return errors.Wrap(err, "failed to disable HW write protect")
		}
	}

	swWPArg := "disable"
	if enableSWWP {
		swWPArg = "enable"
	}
	// This command can return error even on success, so ignore error.
	_, _ = d.Command("ectool", "--name=cros_fp", "flashprotect", swWPArg).Output(ctx)
	testing.Sleep(ctx, 2*time.Second)
	if err := RebootFpmcu(ctx, d, "RW"); err != nil {
		return err
	}

	if enableHWWP {
		hwWPArg = "force_on"
	}
	// Don't use SetStringAndCheck because the state can be "on" after we set "force_on".
	if err := pxy.Servo().SetString(ctx, servo.FWWPState, hwWPArg); err != nil {
		return errors.Wrapf(err, "failed to set HW write protect to %q", hwWPArg)
	}
	// TODO(yichengli): Check the correct flags, which is different for different chips.
	return nil
}

// RebootFpmcu reboots the fingerprint MCU. It does not reboot the AP.
func RebootFpmcu(ctx context.Context, d *dut.DUT, bootTo string) error {
	testing.ContextLog(ctx, "Rebooting FPMCU")
	// This command returns error even on success, so ignore error. b/116396469
	_ = d.Command("ectool", "--name=cros_fp", "reboot_ec").Run(ctx)
	if bootTo == "RO" {
		testing.Sleep(ctx, 500*time.Millisecond)
		err := d.Command("ectool", "--name=cros_fp", "rwsigaction", "abort").Run(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to abort rwsig")
		}
	}
	testing.Sleep(ctx, 2*time.Second)
	firmwareCopy, err := RunningFirmwareCopy(ctx, d)
	if err != nil {
		return err
	}
	if firmwareCopy != bootTo {
		return errors.Errorf("FPMCU booted to %q, expected %q", firmwareCopy, bootTo)
	}
	return nil
}

// RunningFirmwareCopy returns the firmware copy on FPMCU (RO or RW).
func RunningFirmwareCopy(ctx context.Context, d *dut.DUT) (string, error) {
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(versionCmd))
	out, err := d.Command(versionCmd[0], versionCmd[1:]...).Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to query FPMCU version")
	}
	re := regexp.MustCompile(`Firmware copy:\s+(RO|RW)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) != 2 {
		return "", errors.New("cannot find firmware copy string")
	}
	return matches[1], nil
}

// RollbackInfo returns the rollbackinfo of the fingerprint MCU.
func RollbackInfo(ctx context.Context, d *dut.DUT) ([]byte, error) {
	cmd := []string{"ectool", "--name=cros_fp", "rollbackinfo"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmd))
	out, err := d.Command(cmd[0], cmd[1:]...).Output(ctx)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to query FPMCU rollbackinfo")
	}
	return out, nil
}

// CheckRollbackSetToInitialValue checks the anti-rollback block is set to initial values.
func CheckRollbackSetToInitialValue(ctx context.Context, d *dut.DUT) error {
	return CheckRollbackState(ctx, d, 1, 0, 0)
}

// CheckRollbackState checks that the anti-rollback block is set to expected values.
func CheckRollbackState(ctx context.Context, d *dut.DUT, blockID, minVersion, rwVersion int) error {
	rollbackInfo, err := RollbackInfo(ctx, d)
	if err != nil {
		return err
	}
	if !regexp.MustCompile(`Rollback block id:\s+`+strconv.Itoa(blockID)).Match(rollbackInfo) ||
		!regexp.MustCompile(`Rollback min version:\s+`+strconv.Itoa(minVersion)).Match(rollbackInfo) ||
		!regexp.MustCompile(`RW rollback version:\s+`+strconv.Itoa(rwVersion)).Match(rollbackInfo) {
		testing.ContextLogf(ctx, "Rollback info: %q", string(rollbackInfo))
		return errors.New("Rollback not set to initial value")
	}
	return nil
}

// AddEntropy adds entropy to the fingerprint MCU.
func AddEntropy(ctx context.Context, d *dut.DUT, reset bool) error {
	cmd := []string{"ectool", "--name=cros_fp", "addentropy"}
	if reset {
		cmd = append(cmd, "reset")
	}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(cmd))
	return d.Command(cmd[0], cmd[1:]...).Run(ctx)
}
