// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ip contains utility functions to wrap around the ip program.
package ip

import (
	"context"
	"fmt"
	"net"
	"regexp"
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

// LinkState is the type for the interface state from ip command.
type LinkState string

// Possible link states from ip command.
const (
	LinkStateUp      LinkState = "UP"
	LinkStateDown    LinkState = "DOWN"
	LinkStateUnknown LinkState = "UNKNOWN"
)

// showLinkIfaceResult is a collection of sections of the output of an
// "ip link show <iface>" call.
//
// Example output:
// 1: lo@veth1: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
//    link/loopback 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff
type showLinkIfaceResult struct {
	name  string // "lo" in example.
	alias string // "veth1" in example. Will only be set if an alias is present.
	state string // "UNKNOWN" in example
	MAC   string // "00:00:00:00:00:00" in example.
	flags string // "<LOOPBACK,UP,LOWER_UP>" in example.
}

// State returns the operation state of the interface.
func (r *Runner) State(ctx context.Context, iface string) (LinkState, error) {
	ifaceLink, err := r.showLink(ctx, iface)
	if err != nil {
		return "", err
	}
	switch state := LinkState(ifaceLink.state); state {
	case LinkStateUp, LinkStateDown, LinkStateUnknown:
		// Expected state.
		return state, nil
	default:
		return "", errors.Errorf("unexpected link state: %q", state)
	}
}

// MAC returns the MAC address of the interface.
func (r *Runner) MAC(ctx context.Context, iface string) (net.HardwareAddr, error) {
	ifaceLink, err := r.showLink(ctx, iface)
	if err != nil {
		return nil, err
	}
	return net.ParseMAC(ifaceLink.MAC)
}

// Flags returns the flags of the interface.
func (r *Runner) Flags(ctx context.Context, iface string) ([]string, error) {
	ifaceLink, err := r.showLink(ctx, iface)
	if err != nil {
		return nil, err
	}
	flags := strings.Split(strings.Trim(ifaceLink.flags, "<>"), ",")
	return flags, nil
}

// showLink runs `ip link show <iface>` and parses out the interface name,
// state, mac address, flags, and any iface alias from the output.
//
// Example "ip link show" output:
// 1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
//    link/loopback 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff
func (r *Runner) showLink(ctx context.Context, iface string) (*showLinkIfaceResult, error) {
	// Run command and get output lines.
	output, err := r.cmd.Output(ctx, "ip", "link", "show", iface)
	if err != nil {
		return nil, errors.Wrapf(err, `failed to run "ip link show %s"`, iface)
	}
	content := strings.TrimSpace(string(output))
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return nil, errors.Errorf("unexpected lines of results: got %d, want at least 2", len(lines))
	}
	// Parse first line for iface, iface alias, and flags.
	ifaceLink := &showLinkIfaceResult{}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return nil, errors.Errorf(`invalid "ip link show" output: %q`, output)
	}
	outputIfaceWithPossibleAlias := strings.TrimSuffix(fields[1], ":")
	ifaceNames := strings.Split(outputIfaceWithPossibleAlias, "@")
	if len(ifaceNames) > 1 {
		ifaceLink.alias = ifaceNames[1]
	}
	ifaceLink.name = ifaceNames[0]
	if ifaceLink.name != iface {
		return nil, errors.Errorf("unmatched interface name, got %s, want %s", ifaceLink.name, iface)
	}
	ifaceLink.flags = fields[2]
	// Parse the state from the next field after the "state" key.
	for i := 3; i < (len(fields) - 1); i += 2 {
		if fields[i] == "state" {
			ifaceLink.state = fields[i+1]
			break
		}
	}
	if ifaceLink.state == "" {
		return nil, errors.Errorf(`failed to parse state from "ip link show" output: %q`, output)
	}
	// Parse second line for MAC.
	fields = strings.Fields(lines[1])
	if len(fields) < 2 {
		return nil, errors.Errorf(`invalid "ip link show" output: %q`, output)
	}
	ifaceLink.MAC = fields[1]
	return ifaceLink, nil
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

// DeleteIP deletes IPv4/IPv6 settings from the interface (iface).
func (r *Runner) DeleteIP(ctx context.Context, iface string, ip net.IP, maskLen int) error {
	args := []string{"addr", "del", fmt.Sprintf("%s/%d", ip.String(), maskLen), "dev", iface}
	if err := r.cmd.Run(ctx, "ip", args...); err != nil {
		return errors.Wrapf(err, "failed to delete IP address on %s", iface)
	}
	return nil
}

// RouteIP adds ip route to the interface (iface).
func (r *Runner) RouteIP(ctx context.Context, iface string, ip net.IP) error {
	args := []string{"route", "replace", "table", "255", ip.String(), "dev", iface}
	if err := r.cmd.Run(ctx, "ip", args...); err != nil {
		return errors.Wrapf(err, "failed to route IP address on %s", iface)
	}
	return nil
}

// DeleteIPRoute deletes ip route from the interface (iface).
func (r *Runner) DeleteIPRoute(ctx context.Context, iface string, ip net.IP) error {
	args := []string{"route", "del", "table", "255", ip.String(), "dev", iface}
	if err := r.cmd.Run(ctx, "ip", args...); err != nil {
		return errors.Wrapf(err, "failed to delete IP address route on %s", iface)
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
		return errors.Wrapf(err, "failed to set %s up", iface)
	}
	return nil
}

// IsLinkUp queries whether iface is currently up.
func (r *Runner) IsLinkUp(ctx context.Context, iface string) (bool, error) {
	flags, err := r.Flags(ctx, iface)
	if err != nil {
		return false, err
	}
	for _, flag := range flags {
		if flag == "UP" {
			return true, nil
		}
	}
	return false, nil
}

// SetLinkDown brings iface down.
func (r *Runner) SetLinkDown(ctx context.Context, iface string) error {
	if err := r.cmd.Run(ctx, "ip", "link", "set", iface, "down"); err != nil {
		return errors.Wrapf(err, "failed to set %s down", iface)
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

// SetBridge adds the device into the bridge.
func (r *Runner) SetBridge(ctx context.Context, dev, br string) error {
	if err := r.cmd.Run(ctx, "ip", "link", "set", dev, "master", br); err != nil {
		return errors.Wrapf(err, "failed to set %s master %s", dev, br)
	}
	return nil
}

// UnsetBridge unsets the bridge of the device.
func (r *Runner) UnsetBridge(ctx context.Context, dev string) error {
	if err := r.cmd.Run(ctx, "ip", "link", "set", dev, "nomaster"); err != nil {
		return errors.Wrapf(err, "failed to set %s nomaster", dev)
	}
	return nil
}

// LinkWithPrefix shows the device names that start with prefix.
func (r *Runner) LinkWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	output, err := r.cmd.Output(ctx, "ip", "link", "show")
	if err != nil {
		return nil, errors.Wrap(err, `failed to run "ip link show"`)
	}
	content := strings.TrimSpace(string(output))
	var ifaceNamesMatchingPrefix []string
	// Collect the "iface" from lines that start like "2: iface:" or "2: iface@alias:".
	ifaceNameMatcher := regexp.MustCompile("^\\d+:\\s+([^@:]+)@?[^:]*:")
	for _, line := range strings.Split(content, "\n") {
		ifaceNameMatch := ifaceNameMatcher.FindStringSubmatch(line)
		if ifaceNameMatch == nil {
			// This line does not have an iface name, so move on.
			continue
		}
		ifaceName := ifaceNameMatch[1]
		if strings.HasPrefix(ifaceName, prefix) {
			ifaceNamesMatchingPrefix = append(ifaceNamesMatchingPrefix, ifaceName)
		}
	}
	return ifaceNamesMatchingPrefix, nil
}

// IfaceAlias will return the alias of the iface as returned from running
// "ip link show <iface>". If no alias exists, an error will be returned.
func (r *Runner) IfaceAlias(ctx context.Context, iface string) (string, error) {
	ifaceLink, err := r.showLink(ctx, iface)
	if err != nil {
		return "", err
	}
	if ifaceLink.alias == "" {
		return "", errors.Errorf("no alias found for iface %q", iface)
	}
	return ifaceLink.alias, nil
}
