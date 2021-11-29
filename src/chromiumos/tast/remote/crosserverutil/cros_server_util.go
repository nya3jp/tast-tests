// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosserverutil contains utility functions to manage the cros server lifecycle
package crosserverutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// StartCrosServer initiates the cros server process and grpc server on DUT through SSH
func StartCrosServer(ctx context.Context, sshConn *ssh.Conn, outDir string, port int) error {
	args := []string{"-rpctcp", "-port", strconv.Itoa(port)}
	testing.ContextLog(ctx, "Start CrOS server with parameters: ", args)

	// Try to kill any process using the desired port
	StopCrosServer(ctx, sshConn, port)

	// Open up TCP port for incoming traffic
	ipTableArgs := []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
	if err := sshConn.CommandContext(ctx, "iptables", ipTableArgs...).Run(); err != nil {
		return errors.Wrapf(err, "failed to open up TCP port: %d for incoming traffic", port)
	}

	// Start CrOS server as a separate process
	//TODO(jonfan): Directly pipe the output from ssh to testing.contextlog with a marker prefix
	// For cros server log to be effective, changes in cros server has to be made such that
	// log from individual grpc services can be aggregated to the cros server log instead of
	// being exposed through the grp  directional log streaming service
	output, _ := os.Create(filepath.Join(outDir, "cros_server.log"))
	cmd := sshConn.CommandContext(ctx, "/usr/local/libexec/tast/bundles/local_pushed/cros", args...)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to Start CrOS Server with parameter: %v", args)
	}
	return nil
}

// StopCrosServer stops the cros server process and grpc server listening
// on the given port through SSH
func StopCrosServer(ctx context.Context, sshConn *ssh.Conn, port int) error {
	// Get the pid of process using the desired port
	out, err := sshConn.CommandContext(ctx, "lsof", "-t", fmt.Sprintf("-i:%d", port)).CombinedOutput()
	if err != nil {
		return err
	}
	pidStr := strings.TrimRight(string(out), "\r\n")
	if pidStr != "" {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return err
		}
		testing.ContextLogf(ctx, "Kill CrOS process pid: %d port: %d", pid, port)
		if out, err := sshConn.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).CombinedOutput(); err != nil {
			return errors.Wrapf(err, "failed to kill CrOS process pid: %d port: %d StdOut: %v", pid, port, out)
		}
	}
	return nil
}
