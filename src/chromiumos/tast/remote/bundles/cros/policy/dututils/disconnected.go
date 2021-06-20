// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package dututils provides useful DUT related utilities within a small scope.
package dututils

import (
	"context"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// EnsureDisconnected polls and ensures DUT remains disconnected throughout a particular timeout duration.
func EnsureDisconnected(ctx context.Context, d *dut.DUT, timeout time.Duration) error {
	var errTemp = errors.New("DUT still disconnected")

	err := testing.Poll(ctx, func(context.Context) error {
		if err := d.Connect(ctx); err == nil {
			return testing.PollBreak(errors.New("DUT booted up"))
		}
		// The idea is to wait for the whole duration (timeout) to ensure DUT hasn't booted up at any instance throughout the interval.
		return errTemp
	}, &testing.PollOptions{
		Timeout: timeout,
	})

	if err != nil && !errors.Is(err, errTemp) {
		return err
	}
	return nil
}
