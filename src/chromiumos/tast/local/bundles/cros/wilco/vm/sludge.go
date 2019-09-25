// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Const values from /etc/init/wilco_dtc.conf on device
const (
	// WilcoVMCID is the context ID for the VM
	WilcoVMCID         = 512
	wilcoVMJob         = "wilco_dtc"
	wilcoSupportJob    = "wilco_dtc_supportd"
	wilcoVMStartupPort = 7788
)

// SludgeConfig contains different configuration options for starting the Sludge
// VM.
type SludgeConfig struct {
	StartProcesses bool
	TestDbusConfig bool
}

// DefaultSludgeConfig creates and returns a SludgeConfig with the default
// values. These default values are the ones used for the production VM.
func DefaultSludgeConfig() *SludgeConfig {
	c := SludgeConfig{}
	c.StartProcesses = true
	c.TestDbusConfig = false
	return &c
}

// StartSludge starts the upstart process wilco_dtc and wait until the VM is
// fully ready. The parameter start_processes will determine if the
// init processes of the Sludge VM are run (DDV and SA).
func StartSludge(ctx context.Context, config *SludgeConfig) error {
	// Load the vhost-vsock module
	if err := testexec.CommandContext(ctx, "modprobe", "-q", "vhost-vsock").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to load vhost-vsock module")
	}

	server, err := vm.NewStartupListenerServer(wilcoVMStartupPort)
	defer server.Stop()
	if err != nil {
		return errors.Wrap(err, "unable to start VM startup listener gRPC server")
	}

	if err := server.Start(); err != nil {
		return errors.Wrap(err, "unable to start listening server")
	}

	startEnv := fmt.Sprintf("STARTUP_PROCESSES=%t", config.StartProcesses)
	dbusEnv := fmt.Sprintf("TEST_DBUS_CONFIG=%t", config.TestDbusConfig)
	if err := upstart.RestartJob(ctx, wilcoVMJob, startEnv, dbusEnv); err != nil {
		return errors.Wrap(err, "wilco DTC daemon could not start")
	}

	if err := server.WaitReady(ctx); err != nil {
		if stopErr := upstart.StopJob(ctx, wilcoVMJob); stopErr != nil {
			testing.ContextLog(ctx, stopErr.Error())
		}
		return errors.Wrap(err, "timed out waiting for server to start")
	}
	return nil
}

// StopSludge stops the upstart process wilco_dtc.
func StopSludge(ctx context.Context) error {
	if err := upstart.StopJob(ctx, wilcoVMJob); err != nil {
		return errors.Wrap(err, "unable to stop Wilco DTC daemon")
	}
	return nil
}

// StartWilcoSupportDaemon starts the upstart process wilco_dtc_supportd.
func StartWilcoSupportDaemon(ctx context.Context) error {
	if err := upstart.RestartJob(ctx, wilcoSupportJob); err != nil {
		return errors.Wrap(err, "wilco DTC Support daemon could not start")
	}
	return nil
}

// StopWilcoSupportDaemon stops the upstart process wilco_dtc_supportd.
func StopWilcoSupportDaemon(ctx context.Context) error {
	if err := upstart.StopJob(ctx, wilcoSupportJob); err != nil {
		return errors.Wrap(err, "unable to stop Wilco DTC Support daemon")
	}
	return nil
}
