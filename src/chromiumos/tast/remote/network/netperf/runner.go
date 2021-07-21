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

// firewallParams is a set of parameters needed for unblocking test traffic.
var firewallParams = [][]firewall.RuleOption{
	{
		firewall.OptionProto(firewall.L4ProtoTCP),
		firewall.OptionDPortRange(controlPort, dataPort),
		firewall.OptionJumpTarget(firewall.TargetAccept),
		firewall.OptionWait(10),
	},
	{
		firewall.OptionProto(firewall.L4ProtoUDP),
		firewall.OptionDPort(dataPort),
		firewall.OptionJumpTarget(firewall.TargetAccept),
		firewall.OptionWait(10),
	},
}

// collectError collects the error into errorlist.
// This can be useful when you have several steps in a function but cannot early
// return on error. e.g. cleanup functions.
func collectError(ctx context.Context, errorlist *[]error, err error) {
	if err == nil {
		return
	}
	*errorlist = append(*errorlist, err)
}

// newRunner returns a configured instance of runner.
func newRunner(
	ctx context.Context, client, server RunnerHost, cfg Config) (*runner, error) {

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
	netserverPath, err := cmd.FindCmdPath(ctx, npr.server.conn, "netserver")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find command netserver")
	}
	npr.netserverPath = netserverPath
	netperfPath, err := cmd.FindCmdPath(ctx, npr.client.conn, "netperf")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find command netperf")
	}
	npr.netperfPath = netperfPath
	if err = npr.startNetserver(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to start netserver")
	}

	return npr, nil
}

// prepareServer makes sure no old netserver is running on the server.
func prepareServer(ctx context.Context, server RunnerHost) []error {
	// Create a somewhat degraded runner, just to run kill command.
	npr := &runner{}
	npr.server = server

	netserverPath, err := cmd.FindCmdPath(ctx, npr.server.conn, "netserver")
	if err != nil {
		return []error{errors.Wrap(err, "failed to find command netserver")}
	}
	npr.netserverPath = netserverPath
	return npr.stopNetserver(ctx)
}

// stopNetserver kills any existing netserv process on the serving host.
func (r *runner) stopNetserver(ctx context.Context) []error {
	testing.ContextLog(ctx, "Stopping netserver")
	ctx, cancel := context.WithTimeout(ctx, netperfCommandTimeoutMargin)
	defer cancel()
	var errors []error
	err := r.server.conn.CommandContext(ctx, "killall", r.netserverPath).Run()
	collectError(ctx, &errors, err)
	// Turn firwall back on, all 4 rules.
	for _, fw := range firewallParams {
		args := []firewall.RuleOption{firewall.OptionDeleteRule(firewall.InputChain)}
		args = append(args, fw...)
		err = firewall.ExecuteCommand(ctx, r.server.conn, args...)
		collectError(ctx, &errors, err)
	}
	return errors
}

// startNetserver starts netserver and unblock traffic on firewall.
func (r *runner) startNetserver(ctx context.Context) error {
	testing.ContextLog(ctx, "Starting netserver")
	ctx, cancel := context.WithTimeout(ctx, netperfCommandTimeoutMargin)
	defer cancel()
	commandArgs := []string{r.netserverPath, "-p", strconv.Itoa(controlPort)}
	testing.ContextLogf(ctx, "Run: %s %s", "minijail0", strings.Join(commandArgs, " "))

	if err := r.server.conn.CommandContext(ctx, "minijail0", commandArgs...).Run(); err != nil {
		return errors.Wrap(err, "failed to start netserver")
	}

	startupTime := time.Now()
	// Punch 4 holes in the firewall, for all possible traffic type.
	for _, fw := range firewallParams {
		args := []firewall.RuleOption{firewall.OptionAppendRule(firewall.InputChain)}
		args = append(args, fw...)
		if err := firewall.ExecuteCommand(ctx, r.server.conn, args...); err != nil {
			r.stopNetserver(ctx)
			return errors.Wrap(err, "failed to reconfigure firewall")
		}
	}
	// Wait for the netserv to come up. Original autotest waited 3 seconds,
	// probably due to some problems detected empirically.
	elapsed := time.Since(startupTime)
	if elapsed < netservStartupWaitTime {
		testing.Sleep(ctx, netservStartupWaitTime-elapsed)
	}
	return nil
}

// restartNetserver stops and starts the netserver again.
func (r *runner) restartNetserver(ctx context.Context) error {
	if err := r.stopNetserver(ctx); err != nil {
		testing.ContextLog(ctx, "Problems while stopping netserver, err: ", err)
	}
	if err := r.startNetserver(ctx); err != nil {
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
			ctx, r.config.TestTime+netperfCommandTimeoutMargin)
		defer cancel()

		// We need to declare ret here so err won't get shadowed.
		var ret []byte
		// Run the command itself and return result if successful.
		ret, err = r.client.conn.CommandContext(runnerCtx, r.netperfPath, commandArgs...).Output()
		if err == nil {
			// Parse
			Result, err := parseNetperfOutput(
				ctx, r.config.TestType, string(ret), r.config.TestTime)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse netperf result")
			}
			return Result, nil
		}
		testing.ContextLogf(ctx, "Failed to run command netperf: %s", err)

		runnerCtx, cancel = context.WithTimeout(ctx, netperfCommandTimeoutMargin)
		defer cancel()
		// Netperf tends to timeout when unable to connect,
		// make best effort to kill it then.
		_ = r.client.conn.CommandContext(runnerCtx, "killall", r.netperfPath).Run()
		if count > 1 {
			// Restart netserv, let it define timeout by itself.
			if restartErr := r.restartNetserver(ctx); restartErr != nil {
				testing.ContextLog(ctx, "Failed to restart netserv, err: ", restartErr)
			}
		}
	}
	return nil, errors.Wrap(err, "failed to run command netperf")
}

// close netperf runner.
func (r *runner) close(ctx context.Context) {
	r.stopNetserver(ctx)
}
