// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/shutil"
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

func loadLsbRelease(ctx context.Context, d *dut.DUT) (map[string]string, error) {
	out, err := d.Command("cat", "/etc/lsb-release").Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /etc/lsb-release")
	}
	return lsbrelease.Parse(bytes.NewReader(out))
}

func getBoardByLsbRelease(ctx context.Context, d *dut.DUT) (string, error) {
	m, err := loadLsbRelease(ctx, d)
	if err != nil {
		return "", err
	}
	return m[lsbrelease.Board], nil
}

func getBoardByCrosConfig(ctx context.Context, d *dut.DUT) (string, error) {
	out, err := d.Command("cros_config", "/fingerprint", "board").Output(ctx)
	return string(out), err
}

// getFpBoard returns the name of the fingerprint EC on the DUT
func getFpBoard(ctx context.Context, d *dut.DUT) (string, error) {
	// For devices that don't have unibuild support (which is required to
	// use cros_config).
	// TODO(https://crbug.com/1030862): remove when nocturne has cros_config
	// support.
	board, err := getBoardByLsbRelease(ctx, d)
	if err != nil {
		return "", err
	}
	if board == "nocturne" {
		return board + fingerprintBoardNameSuffix, nil
	}

	// Use cros_config to get fingerprint board.
	return getBoardByCrosConfig(ctx, d)
}

func getFpFirmwarePath(ctx context.Context, d *dut.DUT, fpBoard string) (string, error) {
	cmd := "ls " + fingerprintFirmwarePathBase + fpBoard + "*.bin"
	out, err := d.Command("bash", "-c", cmd).Output(ctx)
	if err != nil {
		return "", err
	}
	outStr := strings.TrimSpace(string(out))
	if strings.Contains(outStr, "\n") {
		return "", errors.Errorf("found multiple firmware files for %q", fpBoard)
	}
	return outStr, nil
}

func flashFpFirmware(ctx context.Context, d *dut.DUT) error {
	fpBoard, err := getFpBoard(ctx, d)
	if err != nil {
		return errors.Wrap(err, "failed to get fp board")
	}
	testing.ContextLogf(ctx, "fp board name: %q", fpBoard)

	fpFirmwarePath, err := getFpFirmwarePath(ctx, d, fpBoard)
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

func FpSensor(ctx context.Context, s *testing.State) {
	d := s.DUT()
	// Check version and see if the FPMCU is in a good state.
	versionCmd := []string{"ectool", "--name=cros_fp", "version"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(versionCmd))
	out, err := d.Command(versionCmd[0], versionCmd[1:]...).Output(ctx)

	if err == nil {
		versionOutputFile := "cros_fp_version.txt"
		testing.ContextLogf(ctx, "Writing FP firmware version to %s", versionOutputFile)
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), versionOutputFile), out, 0644); err != nil {
			s.Error("Failed to write FP firmware version to file: ", err)
		}
	} else {
		testing.ContextLogf(ctx, "Failed to query FPMCU version (error: %v). Trying re-flashing FP firmware", err)
		if err := flashFpFirmware(ctx, d); err != nil {
			s.Error("Failed to flash FP firmware: ", err)
		}
	}

	fpencstatusCmd := []string{"ectool", "--name=cros_fp", "fpencstatus"}
	testing.ContextLogf(ctx, "Running command: %q", shutil.EscapeSlice(fpencstatusCmd))
	out, err = d.Command(fpencstatusCmd[0], fpencstatusCmd[1:]...).Output(ctx)

	if err != nil {
		s.Fatalf("%q failed: %v", shutil.EscapeSlice(fpencstatusCmd), err)
	}
	re := regexp.MustCompile("FPMCU encryption status: 0x[a-f0-9]{7}1(.+)FPTPM_seed_set")
	if !re.MatchString(string(out)) {
		s.Errorf("FPTPM seed is not set; output %q doesn't match regex %q", string(out), re)
	}
}
