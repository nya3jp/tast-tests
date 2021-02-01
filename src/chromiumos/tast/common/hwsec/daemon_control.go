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

// DaemonInfo represents the information for a daemon
type DaemonInfo struct {
	Name       string
	DaemonName string
	HasDBus    bool
	DBusName   string
}

// AttestationDaemonInfo represents the DaemonsInfo for attestation.
var AttestationDaemonInfo = &DaemonInfo{
	Name:       "attestation",
	DaemonName: "attestationd",
	HasDBus:    true,
	DBusName:   "org.chromium.Attestation",
}

// CryptohomeDaemonInfo represents the DaemonsInfo for cryptohome.
var CryptohomeDaemonInfo = &DaemonInfo{
	Name:       "cryptohome",
	DaemonName: "cryptohomed",
	HasDBus:    true,
	DBusName:   "org.chromium.Cryptohome",
}

// TPMManagerDaemonInfo represents the DaemonsInfo for tpm_manager.
var TPMManagerDaemonInfo = &DaemonInfo{
	Name:       "tpm_manager",
	DaemonName: "tpm_managerd",
	HasDBus:    true,
	DBusName:   "org.chromium.TpmManager",
}

// TrunksDaemonInfo represents the DaemonsInfo for trunks.
var TrunksDaemonInfo = &DaemonInfo{
	Name:       "trunks",
	DaemonName: "trunksd",
	HasDBus:    true,
	DBusName:   "org.chromium.Trunks",
}

// TcsdDaemonInfo represents the DaemonsInfo for tcsd.
var TcsdDaemonInfo = &DaemonInfo{
	Name:       "tcsd",
	DaemonName: "tcsd",
	HasDBus:    false,
}

// PCAAgentDaemonInfo represents the DaemonsInfo for pca_agent.
var PCAAgentDaemonInfo = &DaemonInfo{
	Name:       "pca_agent",
	DaemonName: "pca_agentd",
	HasDBus:    true,
	DBusName:   "org.chromium.PcaAgent",
}

// FakePCAAgentDaemonInfo represents the DaemonsInfo for fake_pca_agent.
// Note that fake_pca_agentd runs the same service as pca_agentd
var FakePCAAgentDaemonInfo = &DaemonInfo{
	Name:       "fake_pca_agent",
	DaemonName: "fake_pca_agentd",
	HasDBus:    true,
	DBusName:   "org.chromium.PcaAgent",
}

// UIDaemonInfo represents the DaemonsInfo for ui.
var UIDaemonInfo = &DaemonInfo{
	Name:       "ui",
	DaemonName: "ui",
	HasDBus:    false,
}

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
	return dc.waitForDBusService(ctx, CryptohomeDaemonInfo)
}

func (dc *DaemonController) waitForDBusService(ctx context.Context, info *DaemonInfo) error {
	name := info.DBusName
	if _, err := dc.r.Run(ctx, "gdbus", "wait", "--system", name); err != nil {
		return errors.Wrapf(err, "failed to wait for D-Bus service %s", name)
	}
	return nil
}

// Start starts a daemon and waits until the D-Bus interface is responsive if it has D-Bus interface.
func (dc *DaemonController) Start(ctx context.Context, info *DaemonInfo) error {
	if _, err := dc.r.Run(ctx, "start", info.DaemonName); err != nil {
		return errors.Wrapf(err, "failed to start %s", info.Name)
	}
	if info.HasDBus {
		return dc.waitForDBusService(ctx, info)
	}
	return nil
}

// Stop stops a daemon.
func (dc *DaemonController) Stop(ctx context.Context, info *DaemonInfo) error {
	if _, err := dc.r.Run(ctx, "stop", info.DaemonName); err != nil {
		return errors.Wrap(err, "failed to stop fake_pca_agent")
	}
	return nil
}

// Restart restarts a daemon and waits until the D-Bus interface is responsive if it has D-Bus interface.
func (dc *DaemonController) Restart(ctx context.Context, info *DaemonInfo) error {
	if _, err := dc.r.Run(ctx, "restart", info.DaemonName); err != nil {
		return errors.Wrapf(err, "failed to restart %s", info.Name)
	}
	if info.HasDBus {
		return dc.waitForDBusService(ctx, info)
	}
	return nil
}
