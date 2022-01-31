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
	reFirmwareCopy = regexp.MustCompile(`Firmware copy:\s*(RO|RW)`)
	reROVersion    = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion    = regexp.MustCompile(`RW version:\s*(\S+)\s`)
	reECHash       = regexp.MustCompile(`hash:\s*(\S+)\s*`)
	reI2CLookup    = regexp.MustCompile(`Bus: I2C; Port: (\S+); Address: (\S+)`)
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

// I2CLookupInfo is a way to access the port and address of the i2c.
type I2CLookupInfo struct {
	Port    int
	Address int
}

// I2CLookup runs ectool locatechip 0 0 to get Port and Address for I2C.
func (ec *ECTool) I2CLookup(ctx context.Context) (*I2CLookupInfo, error) {
	out, err := ec.Command(ctx, "locatechip", "0", "0").Output(ssh.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "running 'ectool locatechip 0 0' on DUT")
	}
	match := reI2CLookup.FindSubmatch(out)
	if match == nil || len(match) == 0 {
		return nil, errors.Wrapf(err, "lookup for I2C failed, got %q", string(out))
	}

	parsedPort, err := strconv.ParseInt(string(match[1]), 0, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse port val %q as int", string(match[1]))
	}
	parsedAddr, err := strconv.ParseInt(string(match[2]), 0, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse addr val %q as int", string(match[2]))
	}

	return &I2CLookupInfo{Port: int(parsedPort), Address: int(parsedAddr)}, nil
}

// GPIOGetCmd type holds commands for 'ectool gpioget'.
type GPIOGetCmd string

const (
	// ECCbiWp for the 'ectool gpioget ec_cbi_wp' cmd.
	ECCbiWp GPIOGetCmd = "ec_cbi_wp"
)

// GPIOGet runs the 'ectool gpioget' with provided command.
func (ec *ECTool) GPIOGet(ctx context.Context, cmd GPIOGetCmd) (string, error) {
	out, err := ec.Command(ctx, "gpioget", string(cmd)).Output(ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "running 'ectool gpioget %s' on DUT", string(cmd))
	}
	return string(out), nil
}

// I2CCmd type holds commands for interacting with i2c using the ectool
type I2CCmd string

const (
	// I2CRead for the 'ectool i2cread' cmd.
	I2CRead I2CCmd = "i2cread"
	// I2CSpeed for the 'ectool i2cspeed' cmd.
	I2CSpeed I2CCmd = "i2cspeed"
	// I2CWrite for the 'ectool i2cwrite' cmd.
	I2CWrite I2CCmd = "i2cwrite"
	// I2Cxfer for the 'ectool i2cxfer' cmd.
	I2Cxfer I2CCmd = "i2cxfer"
)

// I2C runs the 'ectool i2c*' with provided command and args.
func (ec *ECTool) I2C(ctx context.Context, cmd I2CCmd, args ...string) (string, error) {
	cmdAndArgs := append([]string{string(cmd)}, args...)
	out, err := ec.Command(ctx, cmdAndArgs...).Output(ssh.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "running 'ectool %s' on DUT with args %v", string(cmd), args)
	}
	return string(out), nil
}
