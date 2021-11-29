// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosserverutil contains utility functions to manage the cros server lifecycle
package crosserverutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// Client owns a gRPC connection to the DUT for remote tests to use.
type Client struct {
	// Conn is the gRPC connection. Use this to create gRPC service stubs.
	Conn     *grpc.ClientConn
	sshConn  *ssh.Conn
	hostname string
	port     int
}

func (c *Client) Close(ctx context.Context) error {
	if c.Conn != nil {
		c.Conn.Close()
	}

	return StopCrosServer(ctx, c.sshConn, c.port)
}

func Dial(ctx context.Context, d *dut.DUT, hostname string, port int) (*Client, error) {
	// Start CrOS server
	sshConn := d.Conn()
	if err := StartCrosServer(ctx, sshConn, port); err != nil {
		return nil, errors.Wrap(err, "failed to Start CrOS process")
	}

	// Setup gRPC channel
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", hostname, port), grpc.WithInsecure())
	if err != nil {
		// Best effort to kill the cros server process in case of failure
		StopCrosServer(ctx, sshConn, port)
		return nil, errors.Wrap(err, "failed to Setup gRPC channel")
	}

	return &Client{
		Conn:     conn,
		sshConn:  sshConn,
		hostname: hostname,
		port:     port,
	}, nil
}

// StartCrosServer initiates the cros server process and grpc server on DUT through SSH
func StartCrosServer(ctx context.Context, sshConn *ssh.Conn, port int) error {
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
	cmd := sshConn.CommandContext(ctx, "/usr/local/libexec/tast/bundles/local_pushed/cros", args...)
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
