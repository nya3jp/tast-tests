// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC)
// via the host command `ectool`.

package firmware

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// ECToolName specifies which of the many Chromium EC based MCUs ectool will
// be communicated with.
// Some options are cros_ec, cros_fp, cros_pd, cros_scp, and cros_ish.
type ECToolName string

const (
	// ECToolNameMain selects the main EC using cros_ec.
	ECToolNameMain ECToolName = "cros_ec"
	// ECToolNameFingerprint selects the FPMCU using cros_fp.
	ECToolNameFingerprint ECToolName = "cros_fp"
)

// ECTool allows for interaction with the host command `ectool`.
type ECTool struct {
	dut  *dut.DUT
	name ECToolName
}

// NewECTool creates an ECTool.
func NewECTool(d *dut.DUT, name ECToolName) *ECTool {
	return &ECTool{dut: d, name: name}
}

// Regexps to capture values outputted by ectool version.
var (
	reFirmwareCopy  = regexp.MustCompile(`Firmware copy:\s*(RO|RW)`)
	reROVersion     = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion     = regexp.MustCompile(`RW version:\s*(\S+)\s`)
	reECHash        = regexp.MustCompile(`hash:\s*(\S+)\s*`)
	reTabletModeAng = regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
)

// Command return the prebuilt ssh Command with options and args applied.
func (ec *ECTool) Command(ctx context.Context, args ...string) *ssh.Cmd {
	args = append([]string{"--name=" + string(ec.name)}, args...)
	return ec.dut.Conn().CommandContext(ctx, "ectool", args...)
}

// Version returns the EC version of the active firmware.
func (ec *ECTool) Version(ctx context.Context) (string, error) {
	output, err := ec.Command(ctx, "version").Output(ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "running 'ectool version' on DUT")
	}

	// Parse output to determine whether RO or RW is the active firmware.
	match := reFirmwareCopy.FindSubmatch(output)
	if len(match) == 0 {
		return "", errors.Errorf("did not find firmware copy in 'ectool version' output: %s", output)
	}
	var reActiveFWVersion *regexp.Regexp
	switch string(match[1]) {
	case "RO":
		reActiveFWVersion = reROVersion
	case "RW":
		reActiveFWVersion = reRWVersion
	default:
		return "", errors.Errorf("unexpected match from reFirmwareCopy: got %s; want RO or RW", match[1])
	}

	// Parse either RO version line or RW version line, depending on which is active, to find the active firmware version.
	match = reActiveFWVersion.FindSubmatch(output)
	if len(match) == 0 {
		return "", errors.Errorf("failed to match regexp %s in ectool version output: %s", reActiveFWVersion, output)
	}
	return string(match[1]), nil
}

// Hash returns the EC hash of the active firmware.
func (ec *ECTool) Hash(ctx context.Context) (string, error) {
	out, err := ec.Command(ctx, "echash").Output(ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "running 'ectool echash' on DUT")
	}
	// return string(out), nil

	// Parse output to determine whether RO or RW is the active firmware.
	match := reECHash.FindSubmatch(out)
	if len(match) == 0 {
		return "", errors.Errorf("did not find ec hash 'ectool hash' output: %s", out)
	}

	return string(match[1]), nil
}

// BatteryCutoff runs the ectool batterycutoff command.
func (ec *ECTool) BatteryCutoff(ctx context.Context) error {
	if err := ec.Command(ctx, "batterycutoff").Start(); err != nil {
		return errors.Wrap(err, "running 'ectool batterycutoff' on DUT")
	}
	return nil
}

// SaveTabletModeAngles runs 'ectool motionsense tablet_mode_angle' to save the current angles for tablet mode.
func (ec *ECTool) SaveTabletModeAngles(ctx context.Context) (string, string, error) {
	out, err := ec.Command(ctx, "motionsense", "tablet_mode_angle").Output(ssh.DumpLogOnError)
	if err != nil {
		return "", "", errors.Wrap(err, "running 'ectool motionsense tablet_mode_angle' on DUT")
	}
	matches := reTabletModeAng.FindStringSubmatch(string(out))
	if len(matches) != 3 {
		return "", "", errors.Errorf("unable to retrieve tablet mode angles from 'ectool motionsense tablet_mode_angle' output: %s", out)
	}
	return matches[1], matches[2], nil
}

// ForceTabletModeAngle emulates rotation angles to change DUT's tablet mode setting.
func (ec *ECTool) ForceTabletModeAngle(ctx context.Context, tabletModeAngle, hys string) error {
	if err := ec.Command(ctx, "motionsense", "tablet_mode_angle", tabletModeAngle, hys).Start(); err != nil {
		return errors.Wrap(err, "failed to set tablet_mode_angle to 0")
	}
	return nil
}

// FindBaseGpio iterates through a passed in list of gpios, relevant to control on a detachable base,
// and checks if any one of them exists.
func (ec *ECTool) FindBaseGpio(ctx context.Context, gpios []string) (map[string]string, error) {
	// Create a local map to save the gpios found and their current values from the passed in list.
	results := make(map[string]string)
	for _, name := range gpios {
		// Regular expressions
		reFoundGpio := regexp.MustCompile(fmt.Sprintf(`GPIO\s+%s[^\n\r]*`, name))
		reGpioVal := regexp.MustCompile(`\s+(0|1)`)
		// Check whether the gpio exists, and if it does, also check its value.
		out, err := ec.Command(ctx, "gpioget", name).CombinedOutput()
		if err != nil {
			msg := strings.Split(strings.TrimSpace(string(out)), "\n")
			testing.ContextLogf(ctx, "running 'ectool gpioget %s' on DUT failed: %v, and received: %v", name, err, msg)
		}
		match := reFoundGpio.FindSubmatch(out)
		if len(match) == 0 {
			testing.ContextLogf(ctx, "Did not find gpio with name %s", name)
		} else {
			val := reGpioVal.FindSubmatch(out)
			gpioVal := strings.TrimSpace(string(val[0]))
			testing.ContextLogf(ctx, "Found gpio with name %s, and value: %s", name, gpioVal)
			results[name] = gpioVal
		}
	}
	if len(results) == 0 {
		return nil, errors.New("Unable to find any of the gpios passed in. Consider expanding on the list")
	}
	return results, nil
}
