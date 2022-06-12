// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package utils contains functionality shared by tests that
// exercise firmware restoration.
package utils

import (
	"context"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
)

// SetFWWriteProtect sets a posibility to write to a firmware.
func SetFWWriteProtect(ctx context.Context, h *firmware.Helper, enable bool) error {
	if enable {
		// Enable software WP before hardware WP
		if err := h.Servo.RunECCommand(ctx, "flashwp enable"); err != nil {
			return errors.Wrap(err, "failed to enable flashwp")
		}

		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOn); err != nil {
			return errors.Wrap(err, "failed to enable firmware write protect")
		}
	} else {
		if err := h.Servo.SetFWWPState(ctx, servo.FWWPStateOff); err != nil {
			return errors.Wrap(err, "failed to disable firmware write protect")
		}

		if err := h.Servo.RunECCommand(ctx, "flashwp disable"); err != nil {
			return errors.Wrap(err, "failed to disable flashwp")
		}

	}
	return nil
}
