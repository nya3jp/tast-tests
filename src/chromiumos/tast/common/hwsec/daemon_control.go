// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

/*
This file implements the control of our daemons.
It is meant to have our own implementation so we can support the control in
both local and local test; also, we also wait for D-Bus interfaces to be responsive
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

// NewDaemonController creates a new DaemonController object, where r is used to run the command internally.
func NewDaemonController(r CmdRunner) *DaemonController {
	return &DaemonController{r}
}

// WaitForAllDBusServices waits for all D-Bus services of our interest to be running.
func (dc *DaemonController) WaitForAllDBusServices(ctx context.Context) error {
	// Just waits for cryptohomd because it's at the tail of dependency chain. We might have to change it if any dependency is decoupled.
	return dc.waitForDBusService(ctx, "org.chromium.Cryptohome")
}

func (dc *DaemonController) waitForDBusService(ctx context.Context, name string) error {
	// Without quote, we might find something prefixed by name.
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

// StartCryptohome starts cryptohomed and waits until the D-Bus interface is responsive.
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

// RestartCryptohome restarts cryptohomed and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartCryptohome(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "cryptohomed"); err != nil {
		return errors.Wrap(err, "failed to restart cryptohome")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Cryptohome")
}

// StartAttestation starts attestationd and waits until the D-Bus interface is responsive.
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

// RestartAttestation restarts attestationd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartAttestation(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "attestationd"); err != nil {
		return errors.Wrap(err, "failed to restart attestation")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Attestation")
}

// StartTPMManager starts tpm_managerd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) StartTPMManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to start tpm_manager")
	}
	return dc.waitForDBusService(ctx, "org.chromium.TPMManager")
}

// StopTPMManager stops tpm_managerd.
func (dc *DaemonController) StopTPMManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to stop tpm_manager")
	}
	return nil
}

// RestartTPMManager restarts tpm_managerd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartTPMManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to restart tpm_manager")
	}
	return dc.waitForDBusService(ctx, "org.chromium.TPMManager")
}

// StartPCAAgent starts pca_agentd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) StartPCAAgent(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "pca_agentd"); err != nil {
		return errors.Wrap(err, "failed to start pca_agent")
	}
	return dc.waitForDBusService(ctx, "org.chromium.PcaAgent")
}

// StopPCAAgent stops pca_agentd.
func (dc *DaemonController) StopPCAAgent(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "pca_agentd"); err != nil {
		return errors.Wrap(err, "failed to stop pca_agent")
	}
	return nil
}

// RestartPCAAgent restarts pca_agentd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartPCAAgent(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "pca_agentd"); err != nil {
		return errors.Wrap(err, "failed to restart pca_agent")
	}
	return dc.waitForDBusService(ctx, "org.chromium.PcaAgent")
}

// StartFakePCAAgent starts fake_pca_agentd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) StartFakePCAAgent(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "fake_pca_agentd"); err != nil {
		return errors.Wrap(err, "failed to start fake_pca_agent")
	}
	// Note that fake_pca_agentd runs the same service as pca_agentd.
	return dc.waitForDBusService(ctx, "org.chromium.PcaAgent")
}

// StopFakePCAAgent stops fake_pca_agentd.
func (dc *DaemonController) StopFakePCAAgent(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "fake_pca_agentd"); err != nil {
		return errors.Wrap(err, "failed to stop fake_pca_agent")
	}
	return nil
}

// RestartFakePCAAgent restarts fake_pca_agentd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartFakePCAAgent(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "fake_pca_agentd"); err != nil {
		return errors.Wrap(err, "failed to restart fake_pca_agent")
	}
	// Note that fake_pca_agentd runs the same service as pca_agentd.
	return dc.waitForDBusService(ctx, "org.chromium.PcaAgent")
}
