// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to tell the DUT which firmware to try to boot from.

package firmware

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// Vboot2 determines whether the DUT's current firmware was selected by vboot2.
func Vboot2(ctx context.Context, dut *dut.DUT) (bool, error) {
	output, err := dut.Command("crossystem", "fw_vboot2").Output(ctx)
	if err != nil {
		return false, err
	}
	if string(output) == "1" {
		return true, nil
	} else if string(output) == "0" {
		return false, nil
	}
	return false, errors.Errorf("unexpected fw_vboot2 value: %q", output)
}
