// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC)
// via the host command `ectool`.

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// ECTool allows for interaction with the host command `ectool`.
type ECTool struct {
	dut *dut.DUT
}

// NewECTool creates an ECTool.
func NewECTool(d *dut.DUT) *ECTool {
	return &ECTool{dut: d}
}

// Regexps to capture values outputted by ectool version.
var (
	reFirmwareCopy = regexp.MustCompile(`Firmware copy:\s*(RO|RW)`)
	reROVersion    = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion    = regexp.MustCompile(`RW version:\s*(\S+)\s`)
)

// Version returns the EC version of the active firmware.
func (ec *ECTool) Version(ctx context.Context) (string, error) {
	output, err := ec.dut.Conn().Command("ectool", "version").Output(ctx, ssh.DumpLogOnError)
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
