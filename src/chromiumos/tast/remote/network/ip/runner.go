// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ip contains utility functions to wrap around the ip program.
package ip

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/commander"
	"chromiumos/tast/testing"
)

const (
	ipCmd           = "ip"
	addressTypeMAC  = "link/ether"
	addressTypeIPV4 = "inet"
	addressTypeIPV6 = "inet6"
)

var addressTypes = []string{addressTypeMAC, addressTypeIPV4, addressTypeIPV6}

// AddrInfo contains the addresses of the client.
type AddrInfo struct {
	MAC  string
	IPv4 string
	IPv6 string
}

// Runner is the object used for run ip command.
type Runner struct {
	host commander.Commander
}

// NewRunner creates an ip Runner on the given dut.
func NewRunner(host commander.Commander) *Runner {
	return &Runner{host: host}
}

// IP performs a shell ip command.
func (r *Runner) IP(ctx context.Context, args ...string) ([]byte, error) {
	out, err := r.host.Command(ipCmd, args...).Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "ip command failed")
	} else if len(out) == 0 {
		return nil, errors.New("ip returns empty stdout")
	}
	return out, nil
}

// Addresses returns the addresses (MAC, IP) associated with interface.
func (r *Runner) Addresses(ctx context.Context, iface string) (*AddrInfo, error) {
	addressInfoBytes, err := r.IP(ctx, "addr", "show", iface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the addresses")
	}
	return parseAddrsOutput(ctx, string(addressInfoBytes))
}

// parseAddrsOutput parses the output of `ip addr show  "iface"` command into a AddrInfo struct.
func parseAddrsOutput(ctx context.Context, addressInfo string) (*AddrInfo, error) {
	/*
		"ip addr show %s 2> /dev/null" returns something that looks like:

		2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast
			link/ether ac:16:2d:07:51:0f brd ff:ff:ff:ff:ff:ff
			inet 172.22.73.124/22 brd 172.22.75.255 scope global eth0
			inet6 2620:0:1000:1b02:ae16:2dff:fe07:510f/64 scope global dynamic
				valid_lft 2591982sec preferred_lft 604782sec
			inet6 fe80::ae16:2dff:fe07:510f/64 scope link
				valid_lft forever preferred_lft forever

		We extract the second column from any entry for which the first
		column is an address type we are interested in.  For example,
		for "inet 172.22.73.124/22 ...", we will capture "172.22.73.124/22".
	*/
	clientAddrs := AddrInfo{}
	for _, addressLine := range strings.Split(addressInfo, "\n") {
		addressParts := strings.Split(strings.TrimLeft(addressLine, " "), " ")
		if len(addressParts) < 2 {
			continue
		}
		switch addressParts[0] {
		case addressTypeMAC:
			clientAddrs.MAC = addressParts[1]
		case addressTypeIPV4:
			clientAddrs.IPv4 = addressParts[1]
		case addressTypeIPV6:
			clientAddrs.IPv6 = addressParts[1]
		default:
			testing.ContextLogf(ctx, "Found unexpected address type: got %s, want %v", addressParts[0], addressTypes)
		}
	}

	return &clientAddrs, nil
}
