// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with servo to perform
// power related operations.

package firmware

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// BootDutViaPowerPress performs power button normal press to power on DUT via servo.
func BootDutViaPowerPress(ctx context.Context, h *Helper, dut *dut.DUT) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			return errors.Wrap(err, "failed to power normal press")
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			return errors.Wrap(err, "failed to wait connect DUT")
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute})
}
