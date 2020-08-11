// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC).

package firmware

import (
	"context"
	"regexp"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// Regexps to capture values outputted by ectool version.
var (
	reFirmwareCopy = regexp.MustCompile(`Firmware copy:\s*(RO|RW)`)
	reROVersion    = regexp.MustCompile(`RO version:\s*(\S+)\s`)
	reRWVersion    = regexp.MustCompile(`RW version:\s*(\S+)\s`)
)

// ECVersion queries ectool for the EC version on the active firmware.
func ECVersion(ctx context.Context, d *dut.DUT) (string, error) {
	output, err := d.Command("ectool", "version").Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "running 'ectool version' on dut")
	}
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
	match = reActiveFWVersion.FindSubmatch(output)
	if len(match) == 0 {
		return "", errors.Errorf("failed to match regexp %s in ectool version output: %s", reActiveFWVersion, output)
	}
	return string(match[1]), nil
}
