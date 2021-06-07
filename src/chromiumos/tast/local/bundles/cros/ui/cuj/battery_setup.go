// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/setup"
)

// SetBatteryDischarge forces the battery to discharge by calling the corresponding functions of
// power setup package.
// If there're no batteries found, for example on Chromebox, it does nothing.
func SetBatteryDischarge(ctx context.Context, expectedMaxCapacityDischarge float64) (setup.CleanupCallback, error) {
	batteryPaths, err := power.ListSysfsBatteryPaths(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list battery paths")
	}

	if len(batteryPaths) == 0 {
		// No battery: no cleanup and no discharge.
		return func(context.Context) error { return nil }, nil
	}

	return setup.SetBatteryDischarge(ctx, expectedMaxCapacityDischarge)
}
