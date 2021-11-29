// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crosserverutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/testing"
)

//TODO(jonfan): Move crosserver start/teardown to helper
func StartCrosServer(ctx context.Context, s *testing.State, port int) error {
	sshConn := s.DUT().Conn()
	// TODO(jonfan): Refactor into helper class
	args := []string{"-rpctcp", "-port", strconv.Itoa(port)}
	testing.ContextLogf(ctx, "Start CrOS server with parameters: %v", args)

	// Try to kill any process using the desired port
	StopCrosServer(ctx, s, port)

	// Open up TCP port for incoming traffic
	ipTableArgs := []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"}
	if err := sshConn.CommandContext(ctx, "iptables", ipTableArgs...).Run(); err != nil {
		s.Fatal(fmt.Sprintf("Failed to open up TCP port: %d for incoming traffic", port), err)
	}

	// Start CrOS server as a separate process
	//TODO(jonfan): can we directly pipe to testing.contextlog with a prefix?
	output, _ := os.Create(filepath.Join(s.OutDir(), "cros_server.log"))
	cmd := sshConn.CommandContext(ctx, "/usr/local/libexec/tast/bundles/local_pushed/cros", args...)
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		s.Fatal(fmt.Sprintf("Failed to Start CrOS Server with parameter: %v", args), err)
	}
	return nil
}

func StopCrosServer(ctx context.Context, s *testing.State, port int) error {
	sshConn := s.DUT().Conn()

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
		} else {
			testing.ContextLogf(ctx, "Kill CrOS process pid: %d port: %d", pid, port)
			if out, err := sshConn.CommandContext(ctx, "kill", "-9", strconv.Itoa(pid)).CombinedOutput(); err != nil {
				s.Fatal(fmt.Sprintf("Failed to kill CrOS process pid: %d port: %d StdOut: %v", pid, port, out), err)
			}
		}
	}
	return nil
}
