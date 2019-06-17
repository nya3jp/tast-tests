// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/errors"
)

func StartCryptohome(ctx context.Context) error {
	if _, err := call(ctx, "start", "cryptohomed"); err != nil {
		return errors.New("Failed to start cryptohome")
	}
	return waitForCryptohomeInterface(ctx)
}
func StopCryptohome(ctx context.Context) error {
	if _, err := call(ctx, "stop", "cryptohomed"); err != nil {
		return errors.New("Failed to stop cryptohome")
	}
	return nil
}
func RestartCryptohome(ctx context.Context) error {
	if _, err := call(ctx, "restart", "cryptohomed"); err != nil {
		return errors.New("Failed to restart cryptohome")
	}
	return waitForCryptohomeInterface(ctx)
}

func waitForCryptohomeInterface(ctx context.Context) error {
	tick := time.Tick(100 * time.Millisecond)
	for i := 0; i < 20; i += 1 {
		if _, err := call(
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
