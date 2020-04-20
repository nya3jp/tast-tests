// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wificell

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/services/cros/network"
)

const (
	addressTypeMAC   = "link/ether"
	addressTypeIPV4  = "inet"
	addressTypeIPV6  = "inet6"
	networkInterface = "eth0"
)

var addressTypes = []string{addressTypeMAC, addressTypeIPV4, addressTypeIPV6}

// ClientIface is interface for the client.
type ClientIface struct {
	Name        string
	TestFixture *TestFixture
}

// addresses returns the addresses (MAC, IP) associated with interface.
func (c *ClientIface) addresses(ctx context.Context) (map[string]string, error) {
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
	netIntf := &network.NetInterface{
		NetInterface: c.Name,
	}
	addressInfo, err := c.TestFixture.wifiClient.Addresses(ctx, netIntf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the addresses")
	}

	addresses := make(map[string]string)
	for _, addressLine := range strings.Split(addressInfo.Adds, "\n") {
		addressParts := strings.Split(strings.TrimLeft(addressLine, " "), " ")

		if len(addressParts) < 2 {
			continue
		}

		addressType := addressParts[0]
		addressValue := addressParts[1]

		unexpectedType := true
		for _, t := range addressTypes {
			if addressType == t {
				unexpectedType = false
				break
			}
		}
		if unexpectedType {
			addresses[addressType] = ""
		}

		addresses[addressType] = addressValue
	}

	return addresses, nil
}

// MacAddress returns the mac address of the client.
func (c *ClientIface) MacAddress(ctx context.Context) (string, error) {
	adds, err := c.addresses(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the MAC address")
	}

	return adds[addressTypeMAC], nil
}

// IPv4AddressAndPrefix returns the IPv4 address/prefix, e.g., "192.186.0.1/24".
func (c *ClientIface) IPv4AddressAndPrefix(ctx context.Context) (string, error) {
	adds, err := c.addresses(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the IPv4 address and prefix")
	}

	return adds[addressTypeIPV4], nil
}

// IPv6Address returns the IPv6 address.
func (c *ClientIface) IPv6Address(ctx context.Context) (string, error) {
	adds, err := c.addresses(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get IPv6 address")
	}

	return adds[addressTypeIPV4], nil
}
