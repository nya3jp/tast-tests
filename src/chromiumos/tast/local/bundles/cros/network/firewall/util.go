// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package firewall is a library with utilities for creating an on device firewall
package firewall

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	iptablesCmd  = "/sbin/iptables"
	ip6tablesCmd = "/sbin/ip6tables"
)

// CreateFirewallParams is a list of optional parameters when creating a firewall.
type CreateFirewallParams struct {
	AllowPorts      []string
	AllowInterfaces []string
	AllowProtocols  []string
	BlockPorts      []string
	BlockProtocols  []string
	Timeout         time.Duration
}

// CreateFirewall modifies the iptables to allow traffic on specified ports and
// interfaces and block traffic on specified ports and protocols.
func CreateFirewall(ctx context.Context, params CreateFirewallParams) error {
	cmds := []string{iptablesCmd, ip6tablesCmd}
	timeout := fmt.Sprintf("%.0f", params.Timeout.Seconds())

	// Allow each port and interface on all allowed protocols.
	for _, pr := range params.AllowProtocols {
		// Allow traffic from the specified ports through the firewall.
		for _, po := range params.AllowPorts {
			args := []string{"-I", "OUTPUT", "-p", pr, "-m", "tcp", "--sport", po, "-j", "ACCEPT", "-w", timeout}
			if err := executeIptables(ctx, cmds, args); err != nil {
				return err
			}
		}

		// Allow connections from the allowed interfaces.
		for _, i := range params.AllowInterfaces {
			args := []string{"-I", "FORWARD", "-p", pr, "-i", i, "-j", "ACCEPT", "-w", timeout}
			if err := executeIptables(ctx, cmds, args); err != nil {
				return err
			}
		}
	}

	// Block each port on all blocked protocols.
	for _, pr := range params.BlockProtocols {
		for _, po := range params.BlockPorts {
			// Add this rule with rule-number 2 so that the first rule above, which allows proxy traffic for the OUTPUT chain, has priority.
			args := []string{"-I", "OUTPUT", "2", "-p", pr, "--dport", po, "-j", "REJECT", "-w", timeout}
			if err := executeIptables(ctx, cmds, args); err != nil {
				return err
			}
			// Add this rule with rule-number 2 so that the second rule above, which allows proxy traffic for the FORWARD chain, has priority.
			args = []string{"-I", "FORWARD", "2", "-p", pr, "--dport", po, "-j", "REJECT", "-w", timeout}
			if err := executeIptables(ctx, []string{iptablesCmd}, args); err != nil {
				return err
			}
		}
	}
	return nil
}

func executeIptables(ctx context.Context, cmds, args []string) error {
	for _, cmd := range cmds {
		if err := testexec.CommandContext(ctx, cmd, args...).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to add iptables rule: %s %v", cmd, args)
		}
	}
	return nil
}
