// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firewall

import (
	"context"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/cmd"
	"chromiumos/tast/ssh"
)

// L4Proto is enum type describing protocol to filter.
type L4Proto string

const (
	// L4ProtoTCP Layer 4 protocol: TCP.
	L4ProtoTCP L4Proto = "tcp"
	// L4ProtoUDP Layer 4 protocol: UDP.
	L4ProtoUDP = "udp"
)

// Action is enum type defining firewall action.
type Action string

const (
	// ActionAdd add rule.
	ActionAdd Action = "-A"
	// ActionDel del rule.
	ActionDel = "-D"
)

// Chain with rules
type Chain string

const (
	// InputChain chain located immediately before local reception.
	InputChain Chain = "INPUT"
)

// AcceptRule Fw control routine. Adds/deletes rule that jumps directly
// to "Accept" conclusion.
func AcceptRule(ctx context.Context, conn *ssh.Conn, chain Chain,
	action Action, proto L4Proto, dport int) error {
	iptablesPath, err := cmd.GetCmdPath(ctx, conn, "iptables")
	if err != nil {
		return errors.Wrap(err, "failed to find command iptables")
	}

	commandArgs := []string{string(action), string(chain),
		"-p", string(proto), "-m", string(proto),
		"--dport", strconv.Itoa(dport), "-j", "ACCEPT"}

	return conn.Command(iptablesPath, commandArgs...).Run(ctx)
}
