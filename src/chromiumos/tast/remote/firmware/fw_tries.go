// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to set/check which firmware the DUT should try to boot from.

package firmware

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
)

// SetFWTries sets which firmware (A or B) the DUT should boot to next, and how many times it should try booting ino that firmware.
// If tryCount is 0 for a vboot2 DUT, then fw_try_count will not be modified, only fw_try_next.
func SetFWTries(ctx context.Context, d *dut.DUT, nextFW fwCommon.RWSection, tryCount uint) error {
	if nextFW != fwCommon.RWSectionA && nextFW != fwCommon.RWSectionB {
		return errors.Errorf("unexpected param nextFW: got %s; want A or B", nextFW)
	}
	// Determine crossystem values to set, based on whether DUT uses Vboot2
	r := reporters.New(d)
	vboot2, err := r.Vboot2(ctx)
	if err != nil {
		return errors.Wrap(err, "determining whether DUT uses vboot2")
	}
	crossystemMap := make(map[string]string)
	if vboot2 {
		crossystemMap["fw_try_next"] = string(nextFW)
		if tryCount > 0 {
			crossystemMap["fw_try_count"] = strconv.Itoa(int(tryCount))
		}
	} else {
		// For a vboot1 DUT, tryCount represents fwb_tries.
		// Setting fwb_tries=0 implies that nextFW=A. Likewise, setting fwb_tries>0 implies that nextFW=B.
		// Thus, a combination of nextFW/tryCount = A/>0 or B/0 does not make sense.
		if (nextFW == fwCommon.RWSectionA && tryCount > 0) || (nextFW == fwCommon.RWSectionB && tryCount == 0) {
			return errors.Errorf("unexpected params nextFW/tryCount for vboot1 DUT: want either A/0 or B/>0; got %s/%d", nextFW, tryCount)
		}
		crossystemMap["fwb_tries"] = strconv.Itoa(int(tryCount))
	}
	// Send crossystem command to set values
	crossystemArgs := make([]string, len(crossystemMap))
	i := 0
	for k, v := range crossystemMap {
		crossystemArgs[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	if err := d.Conn().CommandContext(ctx, "crossystem", crossystemArgs...).Run(); err != nil {
		return errors.Wrapf(err, "running crossystem %s", strings.Join(crossystemArgs, " "))
	}
	return nil
}

// CheckFWTries returns an error if unexpected values are found for the DUT's currently booted firmware, next firmware section to try, or number of tries.
// Optionally, currentFW and nextFW can each take fwCommon.RWSectionUnspecified to avoid checking that section.
func CheckFWTries(ctx context.Context, r *reporters.Reporter, expectedCurrentFW, expectedNextFW fwCommon.RWSection, expectedTryCount uint) error {
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
