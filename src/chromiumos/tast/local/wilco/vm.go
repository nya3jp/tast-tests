// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"fmt"
	"time"

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
	DDVDbusTopic       = "com.dell.ddv"
	wilcoVMJob         = "wilco_dtc"
	wilcoVMStartupPort = 7788
)

// VMConfig contains different configuration options for starting the WilcoVM.
type VMConfig struct {
	// StartProcesses will determine if the init processes of the Wilco DTC VM are run (DDV and SA).
	StartProcesses bool
	// TestDBusConfig will start the dbus-daemon with a test configuration.
	TestDBusConfig bool
}

// DefaultVMConfig creates and returns a VMConfig with the default
// values. These default values are the ones used for the production VM.
func DefaultVMConfig() *VMConfig {
	c := VMConfig{}
	c.StartProcesses = true
	c.TestDBusConfig = false
	return &c
}

// VMPID gets the process id of wilco_dtc.
func VMPID(ctx context.Context) (pid int, err error) {
	_, _, pid, err = upstart.JobStatus(ctx, wilcoVMJob)

	return pid, err
}

// StartVM starts the upstart process wilco_dtc and wait until the VM is
// fully ready.
func StartVM(ctx context.Context, config *VMConfig) error {
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
	dbusEnv := fmt.Sprintf("TEST_DBUS_CONFIG=%t", config.TestDBusConfig)
	if err := upstart.RestartJob(ctx, wilcoVMJob, startEnv, dbusEnv); err != nil {
		return errors.Wrap(err, "unable to start wilco_dtc service")
	}

	if err := server.WaitReady(ctx); err != nil {
		if stopErr := upstart.StopJob(ctx, wilcoVMJob); stopErr != nil {
			testing.ContextLog(ctx, stopErr.Error())
		}
		return errors.Wrap(err, "timed out waiting for server to start")
	}
	return nil
}

// StopVM stops the upstart process wilco_dtc.
func StopVM(ctx context.Context) error {
	if err := upstart.StopJob(ctx, wilcoVMJob); err != nil {
		return errors.Wrap(err, "unable to stop wilco_dtc service")
	}
	return nil
}

// WaitForDDVDBus blocks until the ddv dbus service to be available.
func WaitForDDVDBus(ctx context.Context) error {
	// Check if the ctx deadline is set. Calculate how much time is left and
	// use that as the timeout duration. Otherwise default to 5 seconds.
	duration := "5"
	deadline, ok := ctx.Deadline()
	if ok {
		d := deadline.Sub(time.Now()).Round(time.Second)
		duration = fmt.Sprintf("%d", int64(d.Seconds()))
	}

	cmd := vm.CreateVSHCommand(ctx, WilcoVMCID,
		"gdbus", "wait", "--system", "--timeout", duration, DDVDbusTopic)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to check DDV dbus service")
	}
	return nil
}
