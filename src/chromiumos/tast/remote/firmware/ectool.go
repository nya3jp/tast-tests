// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC)
// via the host command `ectool`.

package firmware

import (
	"context"
	"regexp"
	"strconv"

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

// ectool command consts
const (
	GetBacklight string = "pwmgetkblight"
	SetBacklight string = "pwmsetkblight"
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
	reFirmwareCopy        = regexp.MustCompile(`Firmware copy:\s*(RO|RW)`)
	reROVersion           = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion           = regexp.MustCompile(`RW version:\s*(\S+)\s`)
	reKBBacklight         = regexp.MustCompile(`Current keyboard backlight percent:.*(\d+)`)
	reKBBacklightDisabled = regexp.MustCompile(`Keyboard backlight disabled`)
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

// SetKBBacklight sets the DUT keyboards backlight to the given value (0 - 100)
func (ec *ECTool) SetKBBacklight(ctx context.Context, percent int) error {
	stringPercent := strconv.Itoa(percent)
	testing.ContextLog(ctx, "Setting keyboard backlight to: ", stringPercent)
	_, err := ec.Command(ctx, SetBacklight, stringPercent).Output(ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "running 'ectool %v' on DUT", SetBacklight)
	}
	return nil
}

// GetKBBacklight gets the DUT keyboards backlight value in percent (0 - 100)
func (ec *ECTool) GetKBBacklight(ctx context.Context) (int, error) {
	testing.ContextLog(ctx, "Getting current keyboard backlight percent")
	output, err := ec.Command(ctx, GetBacklight).Output(ssh.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrapf(err, "running 'ectool %v' on DUT", GetBacklight)
	}
	percentMatch := reKBBacklight.FindSubmatch(output)
	disabledMatch := reKBBacklightDisabled.FindSubmatch(output)
	if len(percentMatch) == 0 && len(disabledMatch) == 0 {
		return 0, errors.Errorf("did not find backlight value in 'ectool %v' output: %v", GetBacklight, output)
	} else if len(percentMatch) != 0 {
		return strconv.Atoi(string(percentMatch[1]))
	}
	return 0, nil // KB backlight is disabled
}

// HasKBBacklight checks if the DUT keyboards has backlight functionality
func (ec *ECTool) HasKBBacklight(ctx context.Context) bool {
	_, err := ec.Command(ctx, GetBacklight).Output()
	return err == nil
}
