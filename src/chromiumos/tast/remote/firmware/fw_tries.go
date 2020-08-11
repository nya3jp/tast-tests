// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to tell the DUT which firmware to try to boot from.

package firmware

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// RWSection refers to one of the two RW firmware sections installed on a Chromebook.
type RWSection string

const (
	// A is the main firmware section which is loaded unless B is specifically requested.
	A RWSection = "A"

	// B is the alternative firmware section which is loaded when A is unavailable.
	B RWSection = "B"

	// UnspecifiedRWSection is used in CheckFWTries when the test doesn't need to specify which FW is current/next.
	UnspecifiedRWSection RWSection = ""
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
func SetFWTries(ctx context.Context, dut *dut.DUT, nextFW RWSection, count int) error {
	// Determine crossystem values to set, based on whether DUT uses Vboot2
	crossystemMap := make(map[string]string)
	if vboot2, err := Vboot2(ctx, dut); err != nil {
		return errors.Wrap(err, "determining whether dut uses vboot2")
	} else if vboot2 {
		if nextFW != A && nextFW != B {
			return errors.Errorf("unexpected param nextFW: got %q; want A or B", nextFW)
		}
		if count < 0 {
			return errors.Errorf("unexpected param count for vboot2 dut: got %d; want >=0", count)
		}
		crossystemMap["fw_try_next"] = string(nextFW)
		if count > 0 {
			crossystemMap["fw_try_count"] = strconv.Itoa(count)
		}
	} else {
		if nextFW != B {
			return errors.New("vboot1 dut can only set fw tries to B")
		}
		if count <= 0 {
			return errors.Errorf("unexpected param count for vboot1 dut: got %d; want >0", count)
		}
		crossystemMap["fwb_tries"] = strconv.Itoa(count)
	}
	// Send crossystem command to set values
	crossystemArgs := make([]string, len(crossystemMap))
	i := 0
	for k, v := range crossystemMap {
		crossystemArgs[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	if err := dut.Command("crossystem", crossystemArgs...).Run(ctx); err != nil {
		return errors.Wrapf(err, "running crossystem %s", strings.Join(crossystemArgs, " "))
	}
	return nil
}

// CheckFWTries returns an error if unexpected valus are found for the DUT's currently booted firmware, next firmware section to try, or try_count.
// The underlying behavior varies based on whether the DUT is running Vboot1 or Vboot2.
// Optionally, currentFW and nextFW can each take UnspecifiedRWSection to avoid checking that section.
func CheckFWTries(ctx context.Context, dut *dut.DUT, currentFW, nextFW RWSection, count int) error {
	// Setup expected crossystem values
	expectedCSMap := make(map[string]string)
	if currentFW != UnspecifiedRWSection {
		expectedCSMap["mainfw_act"] = string(currentFW)
	}
	if vboot2, err := Vboot2(ctx, dut); err != nil {
		return errors.Wrap(err, "determining whether dut uses vboot2")
	} else if vboot2 {
		if nextFW != UnspecifiedRWSection {
			expectedCSMap["fw_try_next"] = string(nextFW)
		}
		expectedCSMap["fw_try_count"] = strconv.Itoa(count)
	} else {
		if nextFW == A && count > 0 {
			return errors.New("vboot1 dut cannot have count>0 with nextFW==A")
		} else if nextFW == B && count == 0 {
			return errors.New("vboot1 dut cannot have count==0 with nextFW==B")
		}
		expectedCSMap["fwb_tries"] = strconv.Itoa(count)
		if currentFW != UnspecifiedRWSection {
			expectedCSMap["tried_fwb"] = map[RWSection]string{A: "0", B: "1"}[currentFW]
		}
	}

	// Check crossystem values one-by-one
	actualCSMap := make(map[string]string, len(expectedCSMap))
	success := true
	for k, v := range expectedCSMap {
		output, err := dut.Command("crossystem", k).Output(ctx)
		if err != nil {
			return errors.Wrapf(err, "checking crossystem value for key %s", k)
		}
		actualCSMap[k] = string(output)
		if string(output) != v {
			success = false
		}
	}
	if !success {
		return errors.Errorf("unexpected crossystem values: want %+v; got %+v", expectedCSMap, actualCSMap)
	}
	return nil
}
