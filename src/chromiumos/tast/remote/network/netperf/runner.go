// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	dataPort                    = 12866
	controlPort                 = 12865
	netservStartupWaitTime      = "3s"
	netperfCommandTimeoutMargin = "10s"
)

// TestType defines type of tests possible to run in netperf.
type TestType string

const (
	// TestTypeTCPCRR measures how many times we can connect, request a byte,
	// and receive a byte per second.
	TestTypeTCPCRR TestType = "TCP_CRR"
	// TestTypeTCPMaerts : maerts is stream backwards. Measures bitrate of a stream
	// from the netperf server to the client.
	TestTypeTCPMaerts = "TCP_MAERTS"
	// TestTypeTCPRR measures how many times we can request a byte and receive
	// a byte per second.
	TestTypeTCPRR = "TCP_RR"
	// TestTypeTCPSendfile is like a TCP_STREAM test except that the netperf client
	// will use a platform dependent call like sendfile() rather than the simple
	// send() call. This can result in better performance.
	TestTypeTCPSendfile = "TCP_SENDFILE"
	// TestTypeTCPStream measures throughput sending bytes from the client
	// to the server in a TCP stream.
	TestTypeTCPStream = "TCP_STREAM"
	// TestTypeUDPRR measures how many times we can request a byte from the client
	// and receive a byte from the server. If any datagram is dropped, the client
	// or server will block indefinitely. This failure is not evident except
	// as a low transaction rate.
	TestTypeUDPRR = "UDP_RR"
	// TestTypeUDPStream tests UDP throughput sending from the client to the server.
	// There is no flow control here, and generally sending is easier that receiving,
	// so there will be two types of throughput, both receiving and sending.
	TestTypeUDPStream = "UDP_STREAM"
	// TestTypeUDPMaerts isn't a real test type, but we can emulate a UDP stream
	// from the server to the DUT by running the netperf server on the DUT and the
	// client on the server and then doing a UDP_STREAM test.
	TestTypeUDPMaerts = "UDP_MAERTS"
	// Different kinds of tests have different output formats.
)

// Config defines configuration for netperf run.
type Config struct {
	// TestTime how long the test should be run.
	TestTime time.Duration
	// TestType is literally this: test type.
	TestType TestType
	// Reverse: reverse client and server roles.
	Reverse bool
	// HumanReadableTag human readable tag to include in test results.
	HumanReadableTag string
}

// RunnerHost defines host's IP and SSH connection.
type RunnerHost struct {
	conn *ssh.Conn
	ip   string
}

// Runner object
type Runner struct {
	client        RunnerHost
	server        RunnerHost
	config        Config
	netserverPath string
	netperfPath   string
}

// GetCmdPath returns full path for the binary on the given device, defined
// by its SSH connection.
// TODO: move to a better place?
func GetCmdPath(ctx context.Context, conn *ssh.Conn, cmd string) (string, error) {
	res, err := conn.Command("which", cmd).Output(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command 'which %s'", cmd)
	}

	return strings.TrimSpace(string(res)), nil
}

// NewRunner initializes runner and returns Runner object.
func NewRunner(
	ctx context.Context,
	client, server RunnerHost,
	cfg Config) (*Runner, error) {

	npr := &Runner{
		config: cfg,
	}

	// Reverse client and server
	if cfg.Reverse {
		npr.client = server
		npr.server = client
	} else {
		npr.client = client
		npr.server = server
	}
	netserverPath, err := GetCmdPath(ctx, npr.server.conn, "netserver")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find command netserver")
	}
	npr.netserverPath = netserverPath
	netperfPath, err := GetCmdPath(ctx, npr.client.conn, "netperf")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find command netperf")
	}
	npr.netperfPath = netperfPath
	npr.stopNetserv(ctx)
	if err = npr.startNetserv(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start netserver")
	}

	return npr, nil
}

// stopNetserv kills any existing netserv process on the serving host.
func (r *Runner) stopNetserv(ctx context.Context) {
	testing.ContextLog(ctx, "Stopping netserver")
	// Ignoring result on purpose: at start the commands may fail,
	// at teardown these failures do not matter.
	_ = r.server.conn.Command("killall", r.netserverPath).Run(ctx)
	_ = firewallCtl(ctx, r.server.conn, firewallDel, l4ProtoTCP, controlPort)
	_ = firewallCtl(ctx, r.server.conn, firewallDel, l4ProtoTCP, dataPort)
	_ = firewallCtl(ctx, r.server.conn, firewallDel, l4ProtoUDP, controlPort)
	_ = firewallCtl(ctx, r.server.conn, firewallDel, l4ProtoUDP, dataPort)

}

// startNetserv Start netserver and unblock traffic on firewall
func (r *Runner) startNetserv(ctx context.Context) error {
	testing.ContextLog(ctx, "Starting netserver")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	commandArgs := []string{r.netserverPath, "-p", strconv.Itoa(controlPort)}
	testing.ContextLogf(ctx, "Run: %s %s", "minijail0", strings.Join(commandArgs, " "))

	err := r.server.conn.Command("minijail0", commandArgs...).Run(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start netserver")
	}

	startupTime := time.Now()

	err = firewallCtl(ctx, r.server.conn, firewallAdd, l4ProtoTCP, controlPort)
	if err != nil {
		return errors.Wrap(err, "failed to start firewall")
	}
	err = firewallCtl(ctx, r.server.conn, firewallAdd, l4ProtoTCP, dataPort)
	if err != nil {
		return errors.Wrap(err, "failed to start firewall")
	}
	err = firewallCtl(ctx, r.server.conn, firewallAdd, l4ProtoUDP, controlPort)
	if err != nil {
		return errors.Wrap(err, "failed to start firewall")
	}
	err = firewallCtl(ctx, r.server.conn, firewallAdd, l4ProtoUDP, dataPort)
	if err != nil {
		return errors.Wrap(err, "failed to start firewall")
	}
	// Wait for the netserv to come up.
	currTime := time.Now()
	elapsed := currTime.Sub(startupTime)
	startupWaitTime, _ := time.ParseDuration(netservStartupWaitTime)
	if elapsed < startupWaitTime {
		testing.Sleep(ctx, startupWaitTime-elapsed)
	}
	return nil
}

// restartNetserv stops and starts the netserver again.
func (r *Runner) restartNetserv(ctx context.Context) error {
	r.stopNetserv(ctx)
	err := r.startNetserv(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start netserver")
	}
	return nil
}

// Run attempts to run netperf with a given number of retries in case of failure.
func (r *Runner) Run(ctx context.Context, retryCount uint) (*Result, error) {
	// Netperf uses seconds for burst length, extract it.
	testTimeSec := int(r.config.TestTime.Seconds())
	if testTimeSec == 0 {
		return nil, errors.New("run duration must be larger than 0")
	}
	success := false
	var result string
	var err error
	for count := retryCount; count > 0; count-- {
		testing.ContextLogf(ctx, "running: %s -H %s -p %s -t %s -l %s -- -P %s",
			r.netperfPath, r.server.ip, strconv.Itoa(controlPort),
			r.config.TestType, strconv.Itoa(testTimeSec),
			fmt.Sprintf("0,%d", dataPort))

		// Set runner's own timeout based on test time plus guesstimated guard.
		guardTime, _ := time.ParseDuration(netperfCommandTimeoutMargin)
		runnerCtx, cancel := context.WithTimeout(
			ctx, time.Duration(testTimeSec)*time.Second+guardTime)
		defer cancel()

		ret, err := r.client.conn.Command(r.netperfPath,
			"-H", r.server.ip,
			"-p", strconv.Itoa(controlPort),
			"-t", string(r.config.TestType),
			"-l", strconv.Itoa(testTimeSec),
			"--", "-P", fmt.Sprintf("0,%d", dataPort)).Output(runnerCtx)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to run command netperf: %s", err)
			if (count == 1) && !success {
				return nil, errors.Wrap(err, "failed to run command netperf")
			}
			// Netperf tends to timeout when unable to connect, kill it then.
			_ = r.client.conn.Command("killall", r.netperfPath).Run(ctx)
			r.restartNetserv(ctx)
			continue
		}
		success = true

		// testing.ContextLog(ctx, string(res))
		result = string(ret)
		break
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to run command netperf")
	}

	// Parse
	Result := FromResults(
		ctx, r.config.TestType, result, r.config.TestTime)
	testing.ContextLog(ctx, Result)
	return Result, nil
}

// Close netrunner.
func (r *Runner) Close(ctx context.Context) {
	r.stopNetserv(ctx)
}
