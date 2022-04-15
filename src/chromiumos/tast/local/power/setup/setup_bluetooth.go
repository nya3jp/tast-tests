// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/testing"
)

func disableBluetoothAdapter(ctx context.Context, adapter *bluetooth.Adapter) (CleanupCallback, error) {
	prev, err := adapter.Powered(ctx)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "Setting bluetooth adapter %q powered to false from %t", adapter.Path(), prev)
	if err := adapter.SetPowered(ctx, false); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting bluetooth adapter %q powered to %t", adapter.Path(), prev)
		return adapter.SetPowered(ctx, prev)
	}, nil
}

// DisableBluetoothAdapters disables all bluetooth adapters on the DUT.
func DisableBluetoothAdapters(ctx context.Context) (CleanupCallback, error) {
	return Nested(ctx, "disable bluetooth", func(s *Setup) error {
		adapters, err := bluetooth.Adapters(ctx)
		if err != nil {
			return err
		}
		for _, adapter := range adapters {
			s.Add(disableBluetoothAdapter(ctx, adapter))
		}
		return nil
	})
}
