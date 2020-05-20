// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ip contains utility functions to wrap around the ip program.
package ip

import (
	"context"
	"fmt"
	"net"
	"strings"

	"chromiumos/tast/common/network/cmd"
	"chromiumos/tast/errors"
)

// Runner contains methods rely on running "ip" command.
type Runner struct {
	cmd cmd.Runner
}

// NewRunner creates an ip command utility runner.
func NewRunner(cmd cmd.Runner) *Runner {
	return &Runner{
		cmd: cmd,
	}
}

// MAC returns the MAC address of the interface.
func (r *Runner) MAC(ctx context.Context, iface string) (net.HardwareAddr, error) {
	// Let ip print brief output so that we can have less assumption on
	// the output format.
	// The brief format: (ref: print_linkinfo_brief in iproute2)
	// <iface> <operstate> <address> <link_flags>
	// Example:
	// lo               UNKNOWN        00:00:00:00:00:00 <LOOPBACK,UP,LOWER_UP>
	output, err := r.cmd.Output(ctx, "ip", "-br", "link", "show", iface)
	if err != nil {
		return nil, errors.Wrapf(err, `failed to run "ip link show %s"`, iface)
	}
	content := strings.TrimSpace(string(output))
	lines := strings.Split(content, "\n")
	if len(lines) != 1 {
		return nil, errors.Errorf("unexpected lines of results: got %d, want 1", len(lines))
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 4 {
		return nil, errors.Errorf(`invalid "ip -br link show" output: %q`, lines[0])
	}
	if fields[0] != iface {
		return nil, errors.Errorf("unmatched interface name, got %s, want %s", fields[0], iface)
	}
	return net.ParseMAC(fields[2])
}

// SetMAC sets MAC address of iface with command "ip link set $iface address $mac.
func (r *Runner) SetMAC(ctx context.Context, iface string, mac net.HardwareAddr) error {
	if err := r.cmd.Run(ctx, "ip", "link", "set", iface, "address", mac.String()); err != nil {
		return errors.Wrapf(err, "failed to set MAC on %s", iface)
	}
	return nil
}

// AddIPOption is the option type for Runner.AddIP call.
type AddIPOption func(*addIPConfig)

type addIPConfig struct {
	broadcastIP net.IP
}

func (c *addIPConfig) toArgs() []string {
	var args []string
	if c.broadcastIP != nil {
		args = append(args, "broadcast", c.broadcastIP.String())
	}
	return args
}

// AddIPBroadcast returns an AddIPOption setting broadcast IP.
func AddIPBroadcast(broadcastIP net.IP) AddIPOption {
	return func(c *addIPConfig) {
		c.broadcastIP = broadcastIP
	}
}

// AddIP adds IPv4/IPv6 settings to iface.
func (r *Runner) AddIP(ctx context.Context, iface string, ip net.IP, maskLen int, ops ...AddIPOption) error {
	c := &addIPConfig{}
	for _, op := range ops {
		op(c)
	}
	args := []string{"addr", "add", fmt.Sprintf("%s/%d", ip.String(), maskLen), "dev", iface}
	args = append(args, c.toArgs()...)
	if err := r.cmd.Run(ctx, "ip", args...); err != nil {
		return errors.Wrapf(err, "failed to add address on %s", iface)
	}
	return nil
}

// FlushIP flushes IP setting on iface.
func (r *Runner) FlushIP(ctx context.Context, iface string) error {
	if err := r.cmd.Run(ctx, "ip", "addr", "flush", iface); err != nil {
		return errors.Wrapf(err, "failed to flush address of %s", iface)
	}
	return nil
}

// SetLinkUp brings iface up.
func (r *Runner) SetLinkUp(ctx context.Context, iface string) error {
	if err := r.cmd.Run(ctx, "ip", "link", "set", iface, "up"); err != nil {
		err = errors.Wrapf(err, "failed to set %s up", iface)
	}
	return nil
}

// SetLinkDown brings iface down.
func (r *Runner) SetLinkDown(ctx context.Context, iface string) error {
	if err := r.cmd.Run(ctx, "ip", "link", "set", iface, "down"); err != nil {
		err = errors.Wrapf(err, "failed to set %s down", iface)
	}
	return nil
}

// AddLink adds a virtual link of type t.
func (r *Runner) AddLink(ctx context.Context, name, t string, extraArgs ...string) error {
	args := []string{"link", "add", name, "type", t}
	args = append(args, extraArgs...)
	if err := r.cmd.Run(ctx, "ip", args...); err != nil {
		return errors.Wrapf(err, "failed to add link %s of type %s", name, t)
	}
	return nil
}

// DeleteLink deletes a virtual link.
func (r *Runner) DeleteLink(ctx context.Context, name string) error {
	if err := r.cmd.Run(ctx, "ip", "link", "del", name); err != nil {
		return errors.Wrapf(err, "failed to delete link %s", name)
	}
	return nil
}
