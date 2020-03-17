// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/testing"
)

// DisableBluetooth disables the bluetooth adapter on the DUT.
func DisableBluetooth(ctx context.Context) (CleanupCallback, error) {
	const path = "/org/bluez/hci0"
	adapter, err := bluetooth.NewAdapter(ctx, path)
	if err != nil {
		return nil, err
	}
	prev, err := adapter.Powered(ctx)
	if err != nil {
		return nil, err
	}

	testing.ContextLogf(ctx, "Setting bluetooth powered to false from %t", prev)
	if err := adapter.SetPowered(ctx, false); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting bluetooth powered to %t", prev)
		return adapter.SetPowered(ctx, prev)
	}, nil
}
