// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dututils

import (
	"context"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// EnsureDisconnected polls and ensures DUT remains disconnected throughout a particular timeout duration.
func EnsureDisconnected(ctx context.Context, d *dut.DUT, timeout time.Duration) error {
	const interval = time.Second
	for i := 0; i < int(timeout/interval); i++ {
		if err := d.Connect(ctx); err == nil {
			return errors.New("DUT booted up")
		}

		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
