// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC).

package firmware

import (
	"context"
	"fmt"
	"regexp"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// ECVersion queries ectool for the EC version on the active firmware.
func ECVersion(ctx context.Context, d *dut.DUT) (string, error) {
	output, err := d.Command("ectool", "version").Output(ctx)
	if err != nil {
		return "", errors.Wrap(err, "running 'ectool version' on dut")
	}
	match := regexp.MustCompile(`Firmware copy:\s*(RO|RW)`).FindSubmatch(output)
	if len(match) == 0 {
		return "", errors.Errorf("did not find firmware copy in 'ectool version' output: %s", output)
	}
	fwCopy := match[1]
	match = regexp.MustCompile(fmt.Sprintf(`%s version:\s*(\S+)\s`, fwCopy)).FindSubmatch(output)
	if len(match) == 0 {
		return "", errors.Errorf("did not find %s version in 'ectool version' output: %s", fwCopy, output)
	}
	return string(match[1]), nil
}
