// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package reporters

import (
	"context"

	fwCommon "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
)

// CurrentBootMode reports the DUT's active firmware boot mode (normal, dev, rec).
// You must add `SoftwareDeps: []string{"crossystem"},` to your `testing.Test` to use this.
func (r *Reporter) CurrentBootMode(ctx context.Context) (fwCommon.BootMode, error) {
	csMap, err := r.Crossystem(ctx, CrossystemParamDevswBoot, CrossystemParamMainfwType)
	if err != nil {
		return fwCommon.BootModeUnspecified, errors.Wrapf(err, "getting bootmode-related crossystem values (%v, %v)", CrossystemParamDevswBoot, CrossystemParamMainfwType)
	}
	mainfwType := csMap[CrossystemParamMainfwType]
	devswBoot := csMap[CrossystemParamDevswBoot]
	switch mainfwType {
	case "normal":
		if devswBoot == "0" {
			return fwCommon.BootModeNormal, nil
		}
	case "developer":
		if devswBoot == "1" {
			return fwCommon.BootModeDev, nil
		}
	case "recovery":
		return fwCommon.BootModeRecovery, nil
	}
	return fwCommon.BootModeUnspecified, errors.Errorf("unexpected crossystem values: %s=%s, %s=%s", CrossystemParamMainfwType, mainfwType, CrossystemParamDevswBoot, devswBoot)
}

// CheckBootMode verifies that the DUT's active firmware boot mode (normal, dev, rec) matches an expected boot mode.
func (r *Reporter) CheckBootMode(ctx context.Context, expected fwCommon.BootMode) (bool, error) {
	curr, err := r.CurrentBootMode(ctx)
	if err != nil {
		return false, errors.Wrap(err, "determining DUT boot mode")
	}
	return curr == expected, nil
}
