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

	"chromiumos/tast/errors"
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
	if _, err := dc.r.Run(ctx, "/usr/bin/gdbus", "wait", "--system", name); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", name)
	}
	return nil
}

// StartTrunks starts trunksd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) StartTrunks(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "trunksd"); err != nil {
		return errors.Wrap(err, "failed to start trunks")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Trunks")
}

// StopTrunks stops trunksd.
func (dc *DaemonController) StopTrunks(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "trunksd"); err != nil {
		return errors.Wrap(err, "failed to stop trunks")
	}
	return nil
}

// RestartTrunks restarts trunksd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartTrunks(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "trunksd"); err != nil {
		return errors.Wrap(err, "failed to restart trunksd")
	}
	return dc.waitForDBusService(ctx, "org.chromium.Trunks")
}

// StartTcsd starts tcsd.
func (dc *DaemonController) StartTcsd(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "start", "tcsd"); err != nil {
		return errors.Wrap(err, "failed to start tcsd")
	}
	return nil
}

// StopTcsd stops tcsd.
func (dc *DaemonController) StopTcsd(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "stop", "tcsd"); err != nil {
		return errors.Wrap(err, "failed to stop tcsd")
	}
	return nil
}

// RestartTcsd restarts tcsd.
func (dc *DaemonController) RestartTcsd(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "tcsd"); err != nil {
		return errors.Wrap(err, "failed to restart tcsd")
	}
	return nil
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

// StartTpmManager starts tpm_managerd and waits until the D-Bus interface is responsive.
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

// RestartTpmManager restarts tpm_managerd and waits until the D-Bus interface is responsive.
func (dc *DaemonController) RestartTpmManager(ctx context.Context) error {
	if _, err := dc.r.Run(ctx, "restart", "tpm_managerd"); err != nil {
		return errors.Wrap(err, "failed to restart tpm_manager")
	}
	return dc.waitForDBusService(ctx, "org.chromium.TpmManager")
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
