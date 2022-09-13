// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hypervisor

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// IsManatee returns true iff the DUT is running a manatee image.
func IsManatee(ctx context.Context, d *dut.DUT) (bool, error) {
	const cmd = `if [ -f /etc/init/dugong.conf ]; then echo yes; else echo no; fi`
	out, err := d.Conn().CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return false, err
	}
	outstr := strings.TrimSpace(string(out))
	if outstr == "yes" {
		return true, nil
	} else if outstr == "no" {
		return false, nil
	} else {
		return false, errors.Errorf("unexpected output when testing for manatee: %s", out)
	}
}
