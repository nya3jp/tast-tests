// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tablet

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// SetTabletModeEnabled sets tabletModeAngle for provided lidAngle, hysAngle.
func SetTabletModeEnabled(ctx context.Context, lidAngle, hysAngle int) error {
	tabletLidAngle := strconv.Itoa(lidAngle)
	tabletHysAngle := strconv.Itoa(hysAngle)
	if err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", tabletLidAngle, tabletHysAngle).Run(); err != nil {
		return errors.Wrap(err, "failed to execute tablet_mode_angle command")
	}
	return nil
}

// ModeValues returns tabletModeAngle values.
func ModeValues(ctx context.Context) (int, int, error) {
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to retrieve tablet_mode_angle settings")
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		return 0, 0, errors.Wrapf(err, "failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}

	initLidAngle, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to convert initLidAngle to integer")
	}
	initHys, err := strconv.Atoi(string(m[2]))
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to convert initHys to integer")
	}

	return initLidAngle, initHys, nil
}

// EnsureTabletModeEnabled makes sure that the tablet mode state is enabled,
// and returns a function which reverts back to the original state.
func EnsureTabletModeEnabled(ctx context.Context, lidAngle, hysAngle int) (func(ctx context.Context) error, error) {
	// Get the initial tablet_mode_angle settings to restore at the end of test.
	initLidAngle, initHys, err := ModeValues(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get initial tablet_mode_angle values")
	}

	if initLidAngle != lidAngle || initHys != hysAngle {
		if err = SetTabletModeEnabled(ctx, lidAngle, hysAngle); err != nil {
			return nil, errors.Wrap(err, "failed to set DUT to tablet mode")
		}
	}
	// Always revert to the original state; so it can always be back to the original
	// state even when the state changes in another part of the test script.
	return func(ctx context.Context) error {
		return SetTabletModeEnabled(ctx, initLidAngle, initHys)
	}, nil
}
