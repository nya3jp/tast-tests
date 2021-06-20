// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power provides power-related utilities.
package power

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// EnsureDUTisOn checks if DUT is pingable and if not, the function waits till DUT boots up.
func EnsureDUTisOn(ctx context.Context, d *dut.DUT, srvo *servo.Servo) error {
	// Try connecting with a small timeout, if DUT is already up, it will be successful.
	waitCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	if err := d.WaitConnect(waitCtx); err == nil {
		return nil
	}
	// If DUT is not reachable, boot it up.
	if err := srvo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurLongPress); err != nil {
		return errors.Wrap(err, "failed to press power key")
	}

	testing.ContextLog(ctx, "Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}
	testing.ContextLog(ctx, "Reconnected to DUT")
	return nil
}
