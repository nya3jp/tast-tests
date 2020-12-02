// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file reports which firmware the DUT is booted from (A or B),
// which firmware it will try to boot from next, and how many times it will try that firmware.

package reporters

import (
	"context"
	"strconv"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
)

// FWTries returns the currently booted firmware, the next firmware that should be booted, and the try_count.
func (r *Reporter) FWTries(ctx context.Context) (fwCommon.RWSection, fwCommon.RWSection, uint, error) {
	csMap, err := r.Crossystem(ctx, CrossystemParamFWVboot2, CrossystemParamMainfwAct, CrossystemParamFWTryNext, CrossystemParamFWTryCount, CrossystemParamFWBTries)
	if err != nil {
		return fwCommon.RWSectionUnspecified, fwCommon.RWSectionUnspecified, 0, err
	}
	vboot2, err := parseFWVboot2(csMap[CrossystemParamFWVboot2])
	if err != nil {
		return fwCommon.RWSectionUnspecified, fwCommon.RWSectionUnspecified, 0, errors.Wrap(err, "determining whether DUT uses vboot2")
	}
	if vboot2 {
		return fwTriesVboot2(ctx, csMap)
	}
	return fwTriesVboot1(ctx, csMap)
}

// fwTriesVboot2 returns the currently booted firmware, the next firmware to try, and the number of times to try it, for a Vboot2 DUT.
func fwTriesVboot2(ctx context.Context, csMap map[CrossystemParam]string) (fwCommon.RWSection, fwCommon.RWSection, uint, error) {
	currentFW := fwCommon.RWSection(csMap[CrossystemParamMainfwAct])
	nextFW := fwCommon.RWSection(csMap[CrossystemParamFWTryNext])
	tryCount, err := strconv.ParseUint(csMap[CrossystemParamFWTryCount], 10, 32)
	if err != nil {
		return fwCommon.RWSectionUnspecified, fwCommon.RWSectionUnspecified, 0, errors.Wrapf(err, "unexpected crossystem value for %s: got %s; want uint", CrossystemParamFWTryCount, csMap[CrossystemParamFWTryCount])
	}
	return currentFW, nextFW, uint(tryCount), nil
}

// fwTriesVboot1 returns the currently booted firmware, the next firmware to try, and the number of times to try it, for a Vboot1 DUT.
func fwTriesVboot1(ctx context.Context, csMap map[CrossystemParam]string) (fwCommon.RWSection, fwCommon.RWSection, uint, error) {
	currentFW := fwCommon.RWSection(csMap[CrossystemParamMainfwAct])
	tryCount, err := strconv.ParseUint(csMap[CrossystemParamFWBTries], 10, 32)
	if err != nil {
		return fwCommon.RWSectionUnspecified, fwCommon.RWSectionUnspecified, 0, errors.Wrapf(err, "unexpected crossystem value for %s: got %s; want uint", CrossystemParamFWBTries, csMap[CrossystemParamFWBTries])
	}
	var nextFW fwCommon.RWSection
	if tryCount == 0 {
		nextFW = fwCommon.RWSectionA
	} else {
		nextFW = fwCommon.RWSectionB
	}
	return currentFW, nextFW, uint(tryCount), nil
}
