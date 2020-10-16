// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"context"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// L4Proto is enum type describing protocol to filter.
type l4Proto string

const (
	l4ProtoTCP l4Proto = "tcp"
	l4ProtoUDP         = "udp"
)

// FirewallAction is enum type defining fireawll action.
type firewallAction string

const (
	firewallAdd firewallAction = "-A"
	firewallDel                = "-D"
)

// firewallCtl Firewall control routine. Allows blocking/unblocking
// of certain traffic types.
func firewallCtl(ctx context.Context, conn *ssh.Conn,
	action firewallAction, proto l4Proto, dport int) error {
	iptablesPath, err := GetCmdPath(ctx, conn, "iptables")
	if err != nil {
		return errors.Wrap(err, "failed to find command iptables")
	}

	commandArgs := []string{string(action), "INPUT",
		"-p", string(proto), "-m", string(proto),
		"--dport", strconv.Itoa(dport), "-j", "ACCEPT"}

	return conn.Command(iptablesPath, commandArgs...).Run(ctx)
}
