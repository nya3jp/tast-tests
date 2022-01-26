// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crosserverutil contains utility functions to manage the cros server lifecycle
package crosserverutil

import (
	"bufio"
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

// DefaultGRPCServerPort is the default TCP based GRPC Server port for remote testing
const DefaultGRPCServerPort = 4445

// Client owns a gRPC connection to the DUT for remote tests to use.
type Client struct {
	// Conn is the gRPC connection. Use this to create gRPC service stubs.
	Conn      *grpc.ClientConn
	sshConn   *ssh.Conn
	hostname  string
	port      int
	forwarder *ssh.Forwarder
	cmd       *ssh.Cmd
}

// Close shuts down cros server and performs other necessary cleanup.
func (c *Client) Close(ctx context.Context) error {
	if c.Conn != nil {
		c.Conn.Close()
	}
	if c.sshConn != nil {
		if err := c.stopCrosServer(ctx); err != nil {
			return errors.Wrap(err, "failed to stop CrOS server process")
		}
	}
	if c.forwarder != nil {
		if err := c.forwarder.Close(); err != nil {
			return errors.Wrap(err, "failed to close port forwarding")
		}
	}
	return nil
}

// Dial establishes a gRPC connection for a given hostname and port
// The grpc target will be in the form "[hostname]:[port]"
// When useForwarder is true, a local to remote port forwarding will be
// enabled for the desired port
//
// Example without port forwarding:
//  cl, err := crosserverutil.Dial(ctx, s.DUT(), hostname, port, false)
//  if err != nil {
//   	return err
//  }
//  defer cl.Close(ctx)
//  cs := pb.NewChromeServiceClient(cl.Conn)
//  res, err := cs.New(ctx, &pb.NewRequest{});
//
// Example with port forwarding:
//  cl, err := crosserverutil.Dial(ctx, s.DUT(), "localhost", port, true)
//
func Dial(ctx context.Context, d *dut.DUT, hostname string, port int, useForwarder bool) (*Client, error) {
	var err error
	sshConn := d.Conn()
	c := &Client{
		sshConn:  sshConn,
		hostname: hostname,
		port:     port,
	}

	// Best effort to clean up in case of failure
	defer func() {
		if err != nil {
			c.Close(ctx)
		}
	}()

	// Setup forwarder to expose remote gRPC server port through SSH connection
	if useForwarder {
		addr := fmt.Sprintf("localhost:%d", port)
		c.forwarder, err = sshConn.ForwardLocalToRemote("tcp", addr, addr, func(err error) {})
		if err != nil {
			return nil, errors.Wrap(err, "failed to setup port forwarding")
		}
	}

	// Start CrOS server
	if err = c.startCrosServer(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start CrOS server process")
	}

	// Setup gRPC channel
	c.Conn, err = grpc.Dial(fmt.Sprintf("%s:%d", hostname, port), grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup gRPC channel")
	}

	return c, nil
}

// startCrosServer initiates the cros server process and grpc server on DUT through SSH
func (c *Client) startCrosServer(ctx context.Context) error {
	args := []string{"-rpctcp", "-port", strconv.Itoa(c.port)}
	testing.ContextLog(ctx, "Start CrOS server with parameters: ", args)

	// Try to kill any process using the desired port
	if err := c.stopCrosServer(ctx); err != nil {
		return errors.Wrapf(err, "failed to kill existing process using the TCP port: %d", c.port)
	}

	// Open up TCP port for incoming traffic
	ipTableArgs := []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(c.port), "-j", "ACCEPT"}
	if err := c.sshConn.CommandContext(ctx, "iptables", ipTableArgs...).Run(); err != nil {
		return errors.Wrapf(err, "failed to open up TCP port: %d for incoming traffic", c.port)
	}

	// Start CrOS server as a separate process
	// TODO(jonfan): To keep the path of cros private and encapsulated from the users, create a
	// shell script or symlink, e.g. /usr/bin/cros, that resolves the path for cros
	cmd := c.sshConn.CommandContext(ctx, "/usr/local/libexec/tast/bundles/local_pushed/cros", args...)

	// Combine stdout and stderr to a single reader by assigning a single pipe to cmd.Stdout
	// and cmd.Stderr.
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to setup StdOut pipe")
	}
	cmd.Stderr = cmd.Stdout
	stdoutScanner := bufio.NewScanner(cmdReader)

	// Pipe the output from ssh command to testing.Contextlog
	go func() {
		// The command session will close the stderr and stdout upon termination
		// causing the scanner to exit the loop.
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			testing.ContextLog(ctx, "cros: ", line)
		}
	}()

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to start CrOS server with parameter: %v", args)
	}
	c.cmd = cmd

	return nil
}

// stopCrosServer stops the cros server process and grpc server listening
// on the given port through SSH
func (c *Client) stopCrosServer(ctx context.Context) error {
	// Get the pid of process using the desired port
	// lsof return a non-zero code when no process is found. We will ignore the error.
	// GRPC tests leverage port forwarding through SSH tunnel. It introduces a few more
	// processes using the same port. Additional filters are needed to filter out the
	// sshd processes needed for port forwarding.
	out, _ := c.sshConn.CommandContext(ctx, "lsof", "-t", fmt.Sprintf("-i:%d", c.port), "-c", "^sshd").CombinedOutput()

	pidStr := strings.TrimRight(string(out), "\r\n")
	if pidStr == "" {
		return nil
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return err
	}
	testing.ContextLogf(ctx, "Kill CrOS server process pid: %d port: %d", pid, c.port)
	// Cros server process intercepts SIGINT and SIGTERM to gracefully stop gRPC server
	// and the cros process. Killing with SIGTERM provides the client side an opportunity
	// to receive logs during the server shutdown routine.
	if out, err := c.sshConn.CommandContext(ctx, "kill", "-15", strconv.Itoa(pid)).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to kill CrOS server process pid: %d port: %d StdOut: %v", pid, c.port, out)
	}

	// If process using the port is tied to cros command, cmd.Wait() is called as a best effort
	// attempt to receive the remaining logs.
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return nil
}
