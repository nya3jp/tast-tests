// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firewall

import (
	"context"
	"fmt"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// L4Proto is an enum type describing layer 4 protocol to filter.
type L4Proto string

const (
	// L4ProtoTCP Layer 4 protocol: TCP.
	L4ProtoTCP L4Proto = "tcp"
	// L4ProtoUDP Layer 4 protocol: UDP.
	L4ProtoUDP = "udp"
)

// Command is an enum type defining firewall command.
type Command string

const (
	// CommandAppend add rule.
	CommandAppend Command = "-A"
	// CommandDelete del rule.
	CommandDelete = "-D"
)

// Chain with rules
type Chain string

const (
	// InputChain chain for packets meant for delivery to local sockets.
	InputChain Chain = "INPUT"
	// OutputChain chain for locally-generated packets.
	OutputChain Chain = "OUTPUT"
	// ForwardChain chain for packets being routed through the box.
	ForwardChain Chain = "FORWARD"
)

// Target is an enum type defining rule target.
type Target string

const (
	// TargetAccept accepts packet and stops processing rules.
	TargetAccept Target = "ACCEPT"
	// TargetDrop drops packet silently and stops processing rules.
	TargetDrop Target = "DROP"
	// There are other targets possible, extend enum if necessary.
)

// RuleOption is used to provide extra options for iptables to filter by.
type RuleOption func(*[]string)

// ExecuteCommand Adds/deletes an iptables rule.
func ExecuteCommand(ctx context.Context, conn *ssh.Conn, ruleOpt ...RuleOption) error {
	iptablesPath, err := cmd.FindCmdPath(ctx, conn, "iptables")
	if err != nil {
		return errors.Wrap(err, "failed to find command iptables")
	}

	var commandArgs []string
	for _, opt := range ruleOpt {
		opt(&commandArgs)
	}

	return conn.CommandContext(ctx, iptablesPath, commandArgs...).Run()
}

// OptionAppendRule appends a new rule to a given chain.
func OptionAppendRule(chain Chain) RuleOption {
	return func(args *[]string) {
		*args = append(*args, string(CommandAppend), string(chain))
	}
}

// OptionDeleteRule deletes a rule from a given chain.
func OptionDeleteRule(chain Chain) RuleOption {
	return func(args *[]string) {
		*args = append(*args, string(CommandDelete), string(chain))
	}
}

// OptionProto sets up the Layer4 protocol option.
func OptionProto(proto L4Proto) RuleOption {
	return func(args *[]string) {
		*args = append(*args, "-p", string(proto))
	}
}

// OptionDPort sets up the destination port option to a single value.
func OptionDPort(port int) RuleOption {
	return func(args *[]string) {
		*args = append(*args, "--dport", strconv.Itoa(port))
	}
}

// OptionDPortRange sets up the destination port option to a value range.
func OptionDPortRange(portFrom, portTo int) RuleOption {
	return func(args *[]string) {
		*args = append(*args, "--dport", fmt.Sprintf("%d:%d", portFrom, portTo))
	}
}

// OptionJumpTarget sets up the target option to request jump to a new chain.
func OptionJumpTarget(target Target) RuleOption {
	return func(args *[]string) {
		*args = append(*args, "-j", string(target))
	}
}

// OptionWait sets up the wait time for xtables lock.
func OptionWait(seconds int) RuleOption {
	return func(args *[]string) {
		*args = append(*args, "--wait", strconv.Itoa(seconds))
	}
}
