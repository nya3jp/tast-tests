// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tablet

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// SetTabletModeEnabled sets tabletModeAngle for provided lidAngle, hysAngle.
func SetTabletModeEnabled(ctx context.Context, lidAngle, hysAngle string) error {
	if err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle", lidAngle, hysAngle).Run(); err != nil {
		return errors.Wrap(err, "failed to execute tablet_mode_angle command")
	}
	return nil
}

// ModeValues returns tabletModeAngle values.
func ModeValues(ctx context.Context) (string, string, error) {
	re := regexp.MustCompile(`tablet_mode_angle=(\d+) hys=(\d+)`)
	out, err := testexec.CommandContext(ctx, "ectool", "motionsense", "tablet_mode_angle").Output()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to retrieve tablet_mode_angle settings")
	}
	m := re.FindSubmatch(out)
	if len(m) != 3 {
		return "", "", errors.Wrapf(err, "failed to get initial tablet_mode_angle settings: got submatches %+v", m)
	}
	initLidAngle := string(m[1])
	initHys := string(m[2])

	return initLidAngle, initHys, nil
}

// EnsureTabletModeEnabled makes sure that the tablet mode state is enabled,
// and returns a function which reverts back to the original state.
func EnsureTabletModeEnabled(ctx context.Context, lidAngle, hysAngle string) (func(ctx context.Context) error, error) {
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
