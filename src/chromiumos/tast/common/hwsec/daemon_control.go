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
	"strings"
	"time"

	"chromiumos/tast/errors"
)

func waitForDBusService(ctx context.Context, r CmdRunner, name string) error {
	tick := time.Tick(100 * time.Millisecond)
	for i := 0; i < 20; i++ {
		if out, err := r.Run(
			ctx,
			"dbus-send",
			"--system",
			"--dest=org.freedesktop.DBus",
			"--print-reply",
			"/org/freedesktop/DBus",
			"org.freedesktop.DBus.ListNames"); err != nil {
			return err
		} else if strings.Contains(string(out), name) {
			return nil
		}
		<-tick
	}
	return errors.New("Timeout")
}

// StartCryptohome starts cryptohomed and wait for the dbus interface is responsive.
func StartCryptohome(ctx context.Context, r CmdRunner) error {
	if _, err := r.Run(ctx, "start", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to start cryptohome")
	}
	return waitForDBusService(ctx, r, "\"org.chromium.Cryptohome\"")
}

// StopCryptohome stops cryptohomed.
func StopCryptohome(ctx context.Context, r CmdRunner) error {
	if _, err := r.Run(ctx, "stop", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to stop cryptohome")
	}
	return nil
}

// RestartCryptohome restarts cryptohomed and wait for the dbus interface is responsive.
func RestartCryptohome(ctx context.Context, r CmdRunner) error {
	if _, err := r.Run(ctx, "restart", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to restart cryptohome")
	}
	return waitForDBusService(ctx, r, "\"org.chromium.Cryptohome\"")
}

// Different from cyrptohomed, attestationd service starts very fast; so far it doesn't seem
// to be necessary to poll the service.

// StartAttestation starts attestationd.
func StartAttestation(ctx context.Context, r CmdRunner) error {
	if _, err := r.Run(ctx, "start", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to start attestation")
	}
	return waitForDBusService(ctx, r, "attestation")
}

// StopAttestation stops attestationd.
func StopAttestation(ctx context.Context, r CmdRunner) error {
	if _, err := r.Run(ctx, "stop", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to stop attestation")
	}
	return nil
}

// RestartAttestation restarts attestationd and wait for the dbus interface is responsive.
func RestartAttestation(ctx context.Context, r CmdRunner) error {
	if _, err := r.Run(ctx, "restart", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to restart attestation")
	}
	return waitForDBusService(ctx, r, "attestation")
}
