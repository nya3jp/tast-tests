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
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/remote/network/firewall"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// RunnerHost defines host's IP and SSH connection.
type RunnerHost struct {
	conn *ssh.Conn
	ip   string
}

// runner object
type runner struct {
	client        RunnerHost
	server        RunnerHost
	config        Config
	netserverPath string
	netperfPath   string
}

var firewallParams = []struct {
	proto firewall.L4Proto
	port  int
}{
	{firewall.L4ProtoTCP, controlPort},
	{firewall.L4ProtoTCP, dataPort},
	{firewall.L4ProtoUDP, controlPort},
	{firewall.L4ProtoUDP, dataPort},
}

// collectFirstErr collects the first error into firstErr.
// This can be useful when you have several steps in a function but cannot early
// return on error. e.g. cleanup functions.
func collectFirstErr(ctx context.Context, firstErr *error, err error) {
	if err == nil {
		return
	}
	if *firstErr == nil {
		*firstErr = err
	}
}

// newRunner returns a configured instance of runner.
func newRunner(
	ctx context.Context,
	client, server RunnerHost,
	cfg Config, firstRun bool) (*runner, error) {

	npr := &runner{
		config: cfg,
	}

	// Reverse client and server.
	if cfg.Reverse {
		npr.client = server
		npr.server = client
	} else {
		npr.client = client
		npr.server = server
	}
	netserverPath, err := cmd.GetCmdPath(ctx, npr.server.conn, "netserver")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find command netserver")
	}
	npr.netserverPath = netserverPath
	netperfPath, err := cmd.GetCmdPath(ctx, npr.client.conn, "netperf")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find command netperf")
	}
	npr.netperfPath = netperfPath
	// We stop netserver only if runner hasn't been run in this session before,
	// to make sure no previous instance is running.
	if firstRun {
		npr.stopNetserv(ctx)
	}
	if err = npr.startNetserv(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start netserver")
	}

	return npr, nil
}

// stopNetserv kills any existing netserv process on the serving host.
func (r *runner) stopNetserv(ctx context.Context) error {
	testing.ContextLog(ctx, "Stopping netserver")
	var firstErr error
	err := r.server.conn.Command("killall", r.netserverPath).Run(ctx)
	collectFirstErr(ctx, &firstErr, err)
	// Turn firwall back on, all 4 rules.
	for _, fw := range firewallParams {
		err = firewall.AcceptRule(ctx, r.server.conn,
			firewall.InputChain, firewall.ActionDel, fw.proto, fw.port)
		collectFirstErr(ctx, &firstErr, err)
	}
	return firstErr
}

// startNetserv Start netserver and unblock traffic on firewall
func (r *runner) startNetserv(ctx context.Context) error {
	testing.ContextLog(ctx, "Starting netserver")
	ctx, cancel := context.WithTimeout(ctx, netperfCommandTimeoutMargin)
	defer cancel()
	commandArgs := []string{r.netserverPath, "-p", strconv.Itoa(controlPort)}
	testing.ContextLogf(ctx, "Run: %s %s", "minijail0", strings.Join(commandArgs, " "))

	if err := r.server.conn.Command("minijail0", commandArgs...).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to start netserver")
	}

	startupTime := time.Now()
	// Punch 4 holes in the firewall, for all possible traffic type.
	for _, fw := range firewallParams {
		if err := firewall.AcceptRule(ctx, r.server.conn,
			firewall.InputChain, firewall.ActionAdd, fw.proto, fw.port); err != nil {
			r.stopNetserv(ctx)
			return errors.Wrap(err, "failed to weaken firewall")
		}
	}
	// Wait for the netserv to come up. Original autotest waited 3 seconds,
	// probably due to some problems detected empirically.
	currTime := time.Now()
	elapsed := currTime.Sub(startupTime)
	if elapsed < netservStartupWaitTime {
		testing.Sleep(ctx, netservStartupWaitTime-elapsed)
	}
	return nil
}

// restartNetserv stops and starts the netserver again.
func (r *runner) restartNetserv(ctx context.Context) error {
	r.stopNetserv(ctx)
	err := r.startNetserv(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start netserver")
	}
	return nil
}

// run attempts to run netperf with a given number of retries in case of failure.
func (r *runner) run(ctx context.Context, retryCount uint) (*Result, error) {
	// Netperf uses seconds for burst length, extract it.
	testTimeSec := int(r.config.TestTime.Seconds())
	if testTimeSec == 0 {
		return nil, errors.New("run duration must be larger than 0")
	}
	var err error
	for count := retryCount; count > 0; count-- {
		commandArgs := []string{"-H", r.server.ip,
			"-p", strconv.Itoa(controlPort),
			"-t", string(r.config.TestType),
			"-l", strconv.Itoa(testTimeSec),
			"--", "-P", fmt.Sprintf("0,%d", dataPort)}
		testing.ContextLogf(ctx, "Run: %s %s",
			r.netperfPath, strings.Join(commandArgs, " "))

		// Set runner's own timeout based on test time plus guesstimated guard.
		runnerCtx, cancel := context.WithTimeout(
			ctx, time.Duration(testTimeSec)*time.Second+netperfCommandTimeoutMargin)
		defer cancel()

		// Run the command itself and return result if successful.
		ret, err := r.client.conn.Command(r.netperfPath, commandArgs...).Output(runnerCtx)
		if err == nil {
			// Parse
			Result, err := parseNetperfOutput(
				ctx, r.config.TestType, string(ret), r.config.TestTime)
			if err != nil {
				return nil, errors.Wrap(err, "failed to run netperf")
			}
			return Result, nil
		}
		testing.ContextLogf(ctx, "Failed to run command netperf: %s", err)

		runnerCtx, cancel = context.WithTimeout(ctx, netperfCommandTimeoutMargin)
		defer cancel()
		// Netperf tends to timeout when unable to connect,
		// make best effort to kill it then.
		_ = r.client.conn.Command("killall", r.netperfPath).Run(runnerCtx)
		if count > 1 {
			// Restart netserv, let it define timeout by itself.
			r.restartNetserv(ctx)
		}

		continue
	}
	return nil, errors.Wrap(err, "failed to run command netperf")
}

// close netperf runner.
func (r *runner) close(ctx context.Context) {
	r.stopNetserv(ctx)
}
