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
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	// nocturne and nami are special cases and have "_fp" appended.
	// Newer FPMCUs have unique names.
	// See go/cros-fingerprint-firmware-branching-and-signing.
	fingerprintBoardNameSuffix  = "_fp"
	fingerprintFirmwarePathBase = "/opt/google/biod/fw/"
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
