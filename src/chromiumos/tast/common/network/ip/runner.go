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

// ImplementationFeature is a feature whose support may differ between different
// implementations of ip.
type ImplementationFeature int

const (
	// BriefLinkShow refers to the ability to set the "-br" flag for "ip -br link
	// show <iface>"
	BriefLinkShow ImplementationFeature = iota
)

// Runner contains methods rely on running "ip" command.
type Runner struct {
	cmd               cmd.Runner
	supportedFeatures map[ImplementationFeature]bool
}

// NewRunner creates an ip command utility runner.
func NewRunner(cmd cmd.Runner) *Runner {
	return &Runner{
		cmd: cmd,
		supportedFeatures: map[ImplementationFeature]bool{
			// Default feature support.
			BriefLinkShow: true,
		},
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

// SetImplementationFeatureSupport sets the given feature as supported or not
// based upon the value of isSupported.
func (r *Runner) SetImplementationFeatureSupport(feature ImplementationFeature, isSupported bool) {
	r.supportedFeatures[feature] = isSupported
}

// State returns the operation state of the interface.
func (r *Runner) State(ctx context.Context, iface string) (LinkState, error) {
	fields, err := r.showLink(ctx, iface)
	if err != nil {
		return "", err
	}
	switch state := LinkState(fields[1]); state {
	case LinkStateUp, LinkStateDown, LinkStateUnknown:
		// Expected state.
		return state, nil
	default:
		return "", errors.Errorf("unexpected link state: %q", state)
	}
}

// MAC returns the MAC address of the interface.
func (r *Runner) MAC(ctx context.Context, iface string) (net.HardwareAddr, error) {
	fields, err := r.showLink(ctx, iface)
	if err != nil {
		return nil, err
	}
	return net.ParseMAC(fields[2])
}

// Flags returns the flags of the interface.
func (r *Runner) Flags(ctx context.Context, iface string) ([]string, error) {
	fields, err := r.showLink(ctx, iface)
	if err != nil {
		return nil, err
	}
	flags := strings.Split(strings.Trim(fields[3], "<>"), ",")
	return flags, nil
}

// showLink runs `ip -br link show <iface>` then splits and validity-checks the output.
func (r *Runner) showLink(ctx context.Context, iface string) ([]string, error) {
	if r.supportedFeatures[BriefLinkShow] {
		return r.showLinkBrief(ctx, iface)
	}
	return r.showLinkFull(ctx, iface)
}

// showLinkBrief runs `ip -br link show <iface>` then splits and validity-checks the output.
func (r *Runner) showLinkBrief(ctx context.Context, iface string) ([]string, error) {
	// Let ip print brief output so that we can have less assumption on
	// the output format.
	// The brief format: (ref: print_linkinfo_brief in iproute2)
	// <iface> <operstate> <address> <link_flags>
	// Example:
	// lo               UNKNOWN        00:00:00:00:00:00 <LOOPBACK,UP,LOWER_UP>
	output, err := r.cmd.Output(ctx, "ip", "-br", "link", "show", iface)
	if err != nil {
		return nil, errors.Wrapf(err, `failed to run "ip -br link show %s"`, iface)
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
	return fields, nil
}

// showLinkFull runs `ip link show <iface>` and parses out the interface name,
// state, mac address, and flags from the output. The result should match what
// showLinkBrief returns, as this is intended to be used when there is no
// support for BriefLinkShow.
//
// Example "ip link show" output:
// 1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
//    link/loopback 00:00:00:00:00:00 brd ff:ff:ff:ff:ff:ff
func (r *Runner) showLinkFull(ctx context.Context, iface string) ([]string, error) {
	var outputIface string
	var state string
	var MAC string
	var flags string

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

	// Parse first line for iface, flags, and state.
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return nil, errors.Errorf(`invalid "ip link show" output: %q`, output)
	}
	outputIface = strings.TrimSuffix(fields[1], ":")
	if outputIface != iface {
		return nil, errors.Errorf("unmatched interface name, got %s, want %s", outputIface, iface)
	}
	flags = fields[2]
	for i := 3; state == "" && i < (len(fields)-1); i++ {
		if fields[i] == "state" {
			state = fields[i+1]
		}
	}
	if state == "" {
		return nil, errors.Errorf(`failed to parse state from "ip link show" output: %q`, output)
	}

	// Parse second line for MAC.
	fields = strings.Fields(lines[1])
	if len(fields) < 2 {
		return nil, errors.Errorf(`invalid "ip link show" output: %q`, output)
	}
	MAC = fields[1]

	// Return values in the same order as the return of showLinkBrief.
	return []string{
		outputIface,
		state,
		MAC,
		flags,
	}, nil
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
	output, err := r.cmd.Output(ctx, "ip", "-brief", "link", "show")
	if err != nil {
		return nil, errors.Wrap(err, `failed to run "ip -brief link show"`)
	}
	content := strings.TrimSpace(string(output))
	var ret []string
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return nil, errors.New(`failed to parse the output of "ip -brief link show": unexpected empty line`)
		}
		if strings.HasPrefix(fields[0], prefix) {
			ret = append(ret, fields[0])
		}
	}
	return ret, nil
}
