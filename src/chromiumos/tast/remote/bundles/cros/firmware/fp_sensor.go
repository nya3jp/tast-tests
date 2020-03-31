// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// nocturne and nami are special cases and have "_fp" appended.
	// Newer FPMCUs have unique names.
	// See go/cros-fingerprint-firmware-branching-and-signing.
	fingerprintBoardNameSuffix  = "_fp"
	fingerprintFirmwarePathBase = "/opt/google/biod/fw/"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpSensor,
		Desc: "Checks that ectool commands for fingerprint sensor behave as expected",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
	})
}

func getBoardByLsbrelease(ctx *context.Context, d *dut.DUT) (string, error) {
	out, err := d.Command("bash", "-c", "cat /etc/lsb-release | grep CHROMEOS_RELEASE_BOARD").Output(*ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read /etc/lsb-release")
	}
	tokens := strings.SplitN(strings.TrimSpace(string(out)), "=", 2)
	return tokens[1], nil
}

func getBoardByCrosConfig(ctx *context.Context, d *dut.DUT) (string, error) {
	out, err := d.Command("cros_config", "/fingerprint", "board").Output(*ctx)
	return string(out), err
}

// getFpBoard returns the name of the fingerprint EC on the DUT
func getFpBoard(ctx *context.Context, d *dut.DUT) (string, error) {
	// For devices that don't have unibuild support (which is required to
	// use cros_config).
	// TODO(https://crbug.com/1030862): remove when nocturne has cros_config
	// support.
	board, err := getBoardByLsbrelease(ctx, d)
	if err != nil {
		return "", err
	}
	if board == "nocturne" {
		return board + fingerprintBoardNameSuffix, nil
	}

	// Use cros_config to get fingerprint board.
	board, err = getBoardByCrosConfig(ctx, d)
	return board, err
}

func getFpFirmwarePath(ctx *context.Context, d *dut.DUT, fpBoard string) (string, error) {
	cmd := "ls " + fingerprintFirmwarePathBase + fpBoard + "*.bin"
	out, err := d.Command("bash", "-c", cmd).Output(*ctx)
	return strings.TrimSpace(string(out)), err
}

func flashFpFirmware(ctx *context.Context, d *dut.DUT) error {
	fpBoard, err := getFpBoard(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get fp board")
	}
	testing.ContextLogf(*ctx, "fp board name: %q", string(fpBoard))

	fpFirmwarePath, err := getFpFirmwarePath(ctx, d, string(fpBoard))
	if err != nil {
		return errors.Wrap(err, "failed to get fp firmware path")
	}
	flashCmd := "flash_fp_mcu " + fpFirmwarePath
	testing.ContextLogf(*ctx, "Running command: %q", flashCmd)
	if err := d.Command("bash", "-c", flashCmd).Run(*ctx); err != nil {
		return errors.Wrap(err, "flash_fp_mcu failed")
	}
	testing.ContextLog(*ctx, "Flashed FP firmware, now rebooting to get seed")
	if err := d.Reboot(*ctx); err != nil {
		return errors.Wrap(err, "failed to reboot DUT")
	}
	return nil
}

func FpSensor(ctx context.Context, s *testing.State) {
	d := s.DUT()
	ectoolCmd := "ectool --name=cros_fp fpencstatus"
	testing.ContextLogf(ctx, "Running command: %q", ectoolCmd)
	out, err := d.Command("bash", "-c", ectoolCmd).Output(ctx)

	if err != nil {
		testing.ContextLog(ctx, "Failed to query FPMCU encryption status. Trying re-flashing FP firmware")
		flashErr := flashFpFirmware(&ctx, d)
		if flashErr != nil {
			s.Errorf("%v", flashErr)
		}
		testing.ContextLogf(ctx, "Retrying command: %q", ectoolCmd)
		out, err = d.Command("bash", "-c", ectoolCmd).Output(ctx)
	}

	exp := regexp.MustCompile("FPMCU encryption status: 0x[a-f0-9]{7}1(.+)FPTPM_seed_set")
	if err != nil {
		s.Errorf("%q failed: %v", ectoolCmd, err)
	} else if !exp.MatchString(string(out)) {
		s.Errorf("FPTPM seed is not set; output %q doesn't match regex %q", string(out), exp)
	}
}
