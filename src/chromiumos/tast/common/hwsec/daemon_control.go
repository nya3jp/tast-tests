// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the control of our daemons.
It is meant to have our own implementation so we can support the control in
both local and local test; also, we also waits dbus interfaces to be responsive
instead of only (re)starting them.
*/

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// StartCryptohome starts cryptohomed and wait for the dbus interface is responsive.
func StartCryptohome(ctx context.Context, s *testing.State) error {
	if _, err := call(ctx, s, "start", "cryptohomed"); err != nil {
		return errors.New("Failed to start cryptohome")
	}
	return waitForCryptohomeInterface(ctx, s)
}

// StopCryptohome stops cryptohomed.
func StopCryptohome(ctx context.Context, s *testing.State) error {
	if _, err := call(ctx, s, "stop", "cryptohomed"); err != nil {
		return errors.New("Failed to stop cryptohome")
	}
	return nil
}

// RestartCryptohome restarts cryptohomed and wait for the dbus interface is responsive.
func RestartCryptohome(ctx context.Context, s *testing.State) error {
	if _, err := call(ctx, s, "restart", "cryptohomed"); err != nil {
		return errors.New("Failed to restart cryptohome")
	}
	return waitForCryptohomeInterface(ctx, s)
}

func waitForCryptohomeInterface(ctx context.Context, s *testing.State) error {
	tick := time.Tick(100 * time.Millisecond)
	for i := 0; i < 20; i++ {
		if _, err := call(
			ctx,
			s,
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
