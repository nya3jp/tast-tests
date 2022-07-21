// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/firewall"
	"chromiumos/tast/ssh"
)

// firewallHelper adds firewall rules and keeps track of them for cleaning up later.
type firewallHelper struct {
	conn  *ssh.Conn
	rules [][]firewall.RuleOption
}

func newFirewallHelper(conn *ssh.Conn) *firewallHelper {
	return &firewallHelper{
		conn:  conn,
		rules: make([][]firewall.RuleOption, 0),
	}
}

func (f *firewallHelper) open(ctx context.Context, cfg *Config) error {
	var proto firewall.RuleOption
	if cfg.isUDP() {
		proto = firewall.OptionProto(firewall.L4ProtoUDP)
	} else {
		proto = firewall.OptionProto(firewall.L4ProtoTCP)
	}

	rules := []firewall.RuleOption{
		firewall.OptionAppendRule(firewall.InputChain),
		proto,
		firewall.OptionDPort(cfg.Port),
		firewall.OptionJumpTarget(firewall.TargetAccept),
		firewall.OptionWait(10),
	}

	if err := firewall.ExecuteCommand(ctx, f.conn, rules...); err != nil {
		return err
	}

	f.rules = append(f.rules, rules[1:])
	return nil
}

func (f *firewallHelper) close(ctx context.Context) error {
	var allErrors error
	for _, fw := range f.rules {
		args := []firewall.RuleOption{firewall.OptionDeleteRule(firewall.InputChain)}
		args = append(args, fw...)
		if err := firewall.ExecuteCommand(ctx, f.conn, args...); err != nil {
			allErrors = errors.Wrapf(allErrors, "failed to configure firewall, %s", err) // NOLINT
		}
	}

	f.rules = make([][]firewall.RuleOption, 0)
	return allErrors
}
