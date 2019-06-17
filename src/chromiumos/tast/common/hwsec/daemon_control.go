// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the control of our daemons.
It is meant to have our own implementation so we can support the control in
both local and local test; also, we also waits dbus interfaces to be responsive
instead of only (re)starting them.

Note that the design of this file is TBD and the current implementation is a temp
solution; once more use cases in the test scripts these functions should be moved
into the helper class properly.
*/

import (
	"context"
	"time"

	"chromiumos/tast/errors"
)

// StartCryptohome starts cryptohomed and wait for the dbus interface is responsive.
func StartCryptohome(ctx context.Context, h *Helper) error {
	if _, err := h.Run(ctx, "start", "cryptohomed"); err != nil {
		return errors.New("Failed to start cryptohome")
	}
	return waitForCryptohomeInterface(ctx, h)
}

// StopCryptohome stops cryptohomed.
func StopCryptohome(ctx context.Context, h *Helper) error {
	if _, err := h.Run(ctx, "stop", "cryptohomed"); err != nil {
		return errors.New("Failed to stop cryptohome")
	}
	return nil
}

// RestartCryptohome restarts cryptohomed and wait for the dbus interface is responsive.
func RestartCryptohome(ctx context.Context, h *Helper) error {
	if _, err := h.Run(ctx, "restart", "cryptohomed"); err != nil {
		return errors.New("Failed to restart cryptohome")
	}
	return waitForCryptohomeInterface(ctx, h)
}

func waitForCryptohomeInterface(ctx context.Context, h *Helper) error {
	tick := time.Tick(100 * time.Millisecond)
	for i := 0; i < 20; i++ {
		if _, err := h.Run(
			ctx,
			"dbus-send",
			"--system",
			"--dest=org.chromium.Cryptohome",
			"--print-reply=literal",
			"/org/chromium/Cryptohome",
			"org.chromium.CryptohomeInterface.GetTpmStatus",
			"array:byte:"); err == nil {
			return nil
		}
		<-tick
	}
	return errors.New("Timeout")
}
