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
	"chromiumos/tast/testing"
)

// DaemonController controls the daemons via upstart commands.
type DaemonController struct {
	r CmdRunner
}

// NewDaemonController creates a new DaemonController object, where |r| is used to run the command internally.
func NewDaemonController(r CmdRunner) *DaemonController {
	return &DaemonController{r}
}

func (dc *DaemonController) waitForDBusService(ctx context.Context, name string) error {
	// Without quote, we might find something prefixed by |name|.
	name = "\"" + name + "\""
	return testing.Poll(ctx, func(ctx context.Context) error {
		if out, err := dc.r.Run(
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
		return errors.New("daemon not up")
	}, &testing.PollOptions{Interval: 100 * time.Millisecond, Timeout: 15 * time.Second})
}

// StartCryptohome starts cryptohomed and wait for the dbus interface is responsive.
func (dc *DaemonController) StartCryptohome(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to start cryptohome")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Cryptohome")
}

// StopCryptohome stops cryptohomed.
func (dc *DaemonController) StopCryptohome(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to stop cryptohome")
	}
	return nil
}

// RestartCryptohome restarts cryptohomed and wait for the dbus interface is responsive.
func (dc *DaemonController) RestartCryptohome(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to restart cryptohome")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Cryptohome")
}

// StartAttestation starts attestationd.
func (dc *DaemonController) StartAttestation(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to start attestation")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Attestation")
}

// StopAttestation stops attestationd.
func (dc *DaemonController) StopAttestation(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to stop attestation")
	}
	return nil
}

// RestartAttestation restarts attestationd and wait for the dbus interface is responsive.
func (dc *DaemonController) RestartAttestation(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to restart attestation")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Attestation")
}

// StartTpmManager starts tpm_managerd and wait for the dbus interface is responsive.
func (dc *DaemonController) StartTpmManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to start tpm_manager")
	}
	return dc.waitForDBusService(ctx, "org.chromium.TpmManager")
}

// StopTpmManager stops tpm_managerd.
func (dc *DaemonController) StopTpmManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to stop tpm_manager")
	}
	return nil
}

// RestartTpmManager restarts tpm_managerd and wait for the dbus interface is responsive.
func (dc *DaemonController) RestartTpmManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to restart tpm_manager")
	}
	return dc.waitForDBusService(ctx, "org.chromium.TpmManager")
}
