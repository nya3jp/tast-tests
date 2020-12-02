// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to tell the DUT which firmware to try to boot from.

package reporters

import (
	"context"
	"strconv"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
)

// Vboot2 determines whether the DUT's current firmware was selected by vboot2.
func (r *Reporter) Vboot2(ctx context.Context) (bool, error) {
	csValue, err := r.CrossystemParam(ctx, CrossystemParamFWVboot2)
	if err != nil {
		return false, err
	}
	if csValue == "1" {
		return true, nil
	} else if csValue == "0" {
		return false, nil
	}
	return false, errors.Errorf("unexpected fw_vboot2 value: want 1 or 0; got %q", csValue)
}

// FWTries returns the currently booted firmware, the next firmware that should be booted, and the try_count.
// The underlying behavior varies based on whether the DUT is running Vboot1 or Vboot2.
func (r *Reporter) FWTries(ctx context.Context) (currentFW, nextFW fwCommon.RWSection, tryCount int, err error) {
	var vboot2 bool
	vboot2, err = r.Vboot2(ctx)
	if err != nil {
		err = errors.Wrap(err, "determining whether DUT uses vboot2")
		return
	}
	var cs map[CrossystemParam]string
	if vboot2 {
		cs, err = r.Crossystem(ctx, CrossystemParamMainfwAct, CrossystemParamFWTryNext, CrossystemParamFWTryCount)
		if err != nil {
			return
		}
		currentFW = fwCommon.RWSection(cs[CrossystemParamMainfwAct])
		nextFW = fwCommon.RWSection(cs[CrossystemParamFWTryNext])
		tryCount, err = strconv.Atoi(cs[CrossystemParamFWTryCount])
		if err != nil {
			err = errors.Wrapf(err, "crossystem value for %s should be an integer", CrossystemParamFWTryCount)
			return
		}
		return
	}
	// Else, vboot1.
	cs, err = r.Crossystem(ctx, CrossystemParamMainfwAct, CrossystemParamFWBTries)
	if err != nil {
		return
	}
	currentFW = fwCommon.RWSection(cs[CrossystemParamMainfwAct])
	tryCount, err = strconv.Atoi(cs[CrossystemParamFWBTries])
	if err != nil {
		err = errors.Wrapf(err, "crossystem value for %s should be an integer", CrossystemParamFWBTries)
		return
	}
	if tryCount == 0 {
		nextFW = fwCommon.RWSectionA
	} else {
		nextFW = fwCommon.RWSectionB
	}
	return
}

// CheckFWTries returns an error if unexpected values are found for the DUT's currently booted firmware, next firmware section to try, or number of tries.
// Optionally, currentFW and nextFW can each take fwCommon.RWSectionUnspecified to avoid checking that section.
func (r *Reporter) CheckFWTries(ctx context.Context, expectedCurrentFW, expectedNextFW fwCommon.RWSection, expectedTryCount int) error {
	actualCurrentFW, actualNextFW, actualTryCount, err := r.FWTries(ctx)
	if err != nil {
		return errors.Wrap(err, "reporting current FW Tries values")
	}
	currentFWOK := (actualCurrentFW == expectedCurrentFW) || (expectedCurrentFW == fwCommon.RWSectionUnspecified)
	nextFWOK := (actualNextFW == expectedNextFW) || (expectedNextFW == fwCommon.RWSectionUnspecified)
	tryCountOK := actualTryCount == expectedTryCount
	if !currentFWOK || !nextFWOK || !tryCountOK {
		return errors.Errorf("unexpected FW Tries values: got currentFW=%s, nextFW=%s, tryCount=%d; want currentFW=%s, nextFW=%s, tryCount=%d", actualCurrentFW, actualNextFW, actualTryCount, expectedCurrentFW, expectedNextFW, expectedTryCount)
	}
	return nil
}
