// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to tell the DUT which firmware to try to boot from.

package firmware

import (
	"context"
	"fmt"
	"strconv"

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

// SetFWTries sets which firmware (A or B) the DUT should boot to next, and how many times it should try booting ino that firmware.
// If count is 0, then fw_try_count will not be modified, only fw_try_next.
func SetFWTries(ctx context.Context, dut *dut.DUT, next string, count int) error {
	// Behavior varies based on whether DUT is running Vboot1 or Vboot2
	if vboot2, err := Vboot2(ctx, dut); err != nil {
		return err
	} else if vboot2 {
		// Validate params for vboot2
		if next != "A" && next != "B" {
			return errors.Errorf("unexpected param next: got %q; want A or B", next)
		}
		if count < 0 {
			return errors.Errorf("unexpected param count: got %d; want >=0", count)
		}
		// Set fw_try_next
		const fwTryNext = "fw_try_next"
		if err := dut.Command("crossystem", fmt.Sprintf("%s=%s", fwTryNext, next)).Run(ctx); err != nil {
			return errors.Wrapf(err, "setting crossystem %q to %q", fwTryNext, next)
		}
		// Set fw_try_count
		if count > 0 {
			const fwTryCount = "fw_try_count"
			if err := dut.Command("crossystem", fmt.Sprintf("%s=%d", fwTryCount, count)).Run(ctx); err != nil {
				return errors.Wrapf(err, "setting crossystem %q to %d", fwTryCount, count)
			}
		}
		return nil
	}
	// Validate params for vboot1
	if next != "B" {
		return errors.New("vboot1 dut can only set fw tries to B")
	}
	if count <= 0 {
		return errors.Errorf("unexpected param count: got %d; want >0", count)
	}
	// Set fwb_tries
	const FWBTries = "fwb_tries"
	if err := dut.Command("crossystem", fmt.Sprintf("%s=%d", FWBTries, count)).Run(ctx); err != nil {
		return errors.Wrapf(err, "setting crossystem %q to %d", FWBTries, count)
	}
	return nil
}

// CheckFWTries verifies the DUT's currently booted firmware and try_count, and returns an error if the expected values are not found.
// The actual behavior varies based on whether the DUT is running Vboot1 or Vboot2.
func CheckFWTries(ctx context.Context, dut *dut.DUT, mainfwAct string, count int) error {
	if mainfwAct != "A" && mainfwAct != "B" {
		return errors.Errorf("unexpected param mainfwAct: got %q; want A or B", mainfwAct)
	}
	// Setup expected crossystem values
	crossystemChecks := map[string]string{"mainfw_act": mainfwAct}
	if vboot2, err := Vboot2(ctx, dut); err != nil {
		return err
	} else if vboot2 {
		crossystemChecks["fw_try_count"] = strconv.Itoa(count)
	} else {
		crossystemChecks["fwb_tries"] = strconv.Itoa(count)
		crossystemChecks["tried_fwb"] = map[string]string{"A": "0", "B": "1"}[mainfwAct]
	}
	// Check crossystem values
	for k, v := range crossystemChecks {
		if output, err := dut.Command("crossystem", k).Output(ctx); err != nil {
			return errors.Wrapf(err, "checking crossystem value for key %s", k)
		} else if string(output) != v {
			return errors.Errorf("unexpected crossystem value for key %s: got %s; want %s", k, output, v)
		}
	}
	return nil
}
