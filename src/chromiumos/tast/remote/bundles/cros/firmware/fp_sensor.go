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
	"chromiumos/tast/testing/hwdep"
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
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

func loadLsbrelease(ctx *context.Context, d *dut.DUT) (map[string]string, error) {
	out, err := d.Command("cat", "/etc/lsb-release").Output(*ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /etc/lsb-release")
	}
	lineRe := regexp.MustCompile(`^([A-Z0-9_]+)\s*=\s*(.*)$`)
	kvs := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		kvs[m[1]] = m[2]
	}
	return kvs, nil
}

func getBoardByLsbrelease(ctx *context.Context, d *dut.DUT) (string, error) {
	m, err := loadLsbrelease(ctx, d)
	if err != nil {
		return "", err
	}
	return m["CHROMEOS_RELEASE_BOARD"], nil
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
	testing.ContextLogf(*ctx, "fp board name: %q", fpBoard)

	fpFirmwarePath, err := getFpFirmwarePath(ctx, d, fpBoard)
	if err != nil {
		return errors.Wrap(err, "failed to get fp firmware path")
	}
	flashCmd := "flash_fp_mcu " + fpFirmwarePath
	testing.ContextLogf(*ctx, "Running command: %q", flashCmd)
	if err := d.Command("bash", "-c", flashCmd).Run(*ctx); err != nil {
		return errors.Wrap(err, "flash_fp_mcu failed")
	}
	testing.ContextLog(*ctx, "Flashed FP firmware, now initializing the entropy")
	if err := d.Command("bio_wash", "--factory_init").Run(*ctx); err != nil {
		return errors.Wrap(err, "failed to initialize entropy after flashing FPMCU")
	}
	testing.ContextLog(*ctx, "Entropy initialized, now rebooting to get seed")
	if err := d.Reboot(*ctx); err != nil {
		return errors.Wrap(err, "failed to reboot DUT")
	}
	return nil
}

func FpSensor(ctx context.Context, s *testing.State) {
	d := s.DUT()
	// Check version and see if the FPMCU is in a good state.
	versionCmd := "ectool --name=cros_fp version"
	testing.ContextLogf(ctx, "Running command: %q", versionCmd)
	out, err := d.Command("bash", "-c", versionCmd).Output(ctx)
	testing.ContextLog(ctx, string(out))

	if err != nil {
		testing.ContextLog(ctx, "Failed to query FPMCU version. Trying re-flashing FP firmware")
		flashErr := flashFpFirmware(&ctx, d)
		if flashErr != nil {
			s.Errorf("%v", flashErr)
		}
	}

	fpencstatusCmd := "ectool --name=cros_fp fpencstatus"
	testing.ContextLogf(ctx, "Running command: %q", fpencstatusCmd)
	out, err = d.Command("bash", "-c", fpencstatusCmd).Output(ctx)

	exp := regexp.MustCompile("FPMCU encryption status: 0x[a-f0-9]{7}1(.+)FPTPM_seed_set")
	if err != nil {
		s.Errorf("%q failed: %v", fpencstatusCmd, err)
	} else if !exp.MatchString(string(out)) {
		s.Errorf("FPTPM seed is not set; output %q doesn't match regex %q", string(out), exp)
	}
}
