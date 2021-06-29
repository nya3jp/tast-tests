// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wilco

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/mdlayher/vsock"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Const values from /etc/init/wilco_dtc.conf on device
const (
	// WilcoVMCID is the context ID for the VM
	WilcoVMCID                      = 512
	DDVDbusTopic                    = "com.dell.ddv"
	wilcoVMJob                      = "wilco_dtc"
	wilcoVMStartupPort              = 7788
	wilcoVMDTCPort                  = 6667
	wilcoVMUIMessageReceiverDTCPort = 6668
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

	startEnv := upstart.WithArg("STARTUP_PROCESSES", strconv.FormatBool(config.StartProcesses))
	dbusEnv := upstart.WithArg("TEST_DBUS_CONFIG", strconv.FormatBool(config.TestDBusConfig))
	if err := upstart.RestartJob(ctx, wilcoVMJob, startEnv, dbusEnv); err != nil {
		return errors.Wrap(err, "unable to start wilco_dtc service")
	}

	if err := server.WaitReady(ctx); err != nil {
		if stopErr := upstart.StopJob(ctx, wilcoVMJob); stopErr != nil {
			testing.ContextLog(ctx, stopErr.Error())
		}
		return errors.Wrap(err, "timed out waiting for server to start")
	}

	if config.StartProcesses {
		for _, port := range []uint32{wilcoVMDTCPort, wilcoVMUIMessageReceiverDTCPort} {
			if err := waitVMGRPCServerReady(ctx, port); err != nil {
				return errors.Wrapf(err, "unable to wait for gRPC server to be ready on %d port", port)
			}
		}
	}

	// Make sure the VM is ready to run commands over vsh.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		cmd := vm.CreateVSHCommand(ctx, WilcoVMCID, "true")
		return cmd.Run(testexec.DumpLogOnError)
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "timed out waiting for VM to be ready")
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

// waitVMGRPCServerReady waits until the gRPC server inside the wilco VM will be ready for incoming messages.
func waitVMGRPCServerReady(ctx context.Context, port uint32) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		vsockHostDialer := func(addr string, duration time.Duration) (net.Conn, error) {
			return vsock.Dial(WilcoVMCID, port)
		}
		conn, err := grpc.DialContext(ctx, "", grpc.WithBlock(), grpc.WithDialer(vsockHostDialer), grpc.WithInsecure(), grpc.WithTimeout(100*time.Millisecond))
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}, &testing.PollOptions{})
}
