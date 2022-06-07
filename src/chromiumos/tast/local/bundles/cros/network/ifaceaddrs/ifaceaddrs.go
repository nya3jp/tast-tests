// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// package ifaceaddrs provides utils to read the IP addresses on an interface
package ifaceaddrs

import (
	"chromiumos/tast/errors"
	"net"
)

// IfaceAddrs represents the IP addresses configured on an interface.
type IfaceAddrs struct {
	// IPv4Addr is the IPv4 address on the interface. There is only one IPv4
	// address on an interface.
	IPv4Addr net.IP
	// IPv6Addrs is the list of IPv6 addresses (excluding link-local address) on
	// the interface.
	IPv6Addrs []net.IP
}

// All returns all addresses (excluding the IPv6 link-local address) on this
// interface.
func (addrs *IfaceAddrs) All() []net.IP {
	var ret []net.IP
	if addrs.IPv4Addr != nil {
		ret = append(ret, addrs.IPv4Addr)
	}
	return append(ret, addrs.IPv6Addrs...)
}

// ReadFromInterface reads the currently configured IP addresses on the
// interface with given name.
func ReadFromInterface(ifname string) (*IfaceAddrs, error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get interface object for %s", ifname)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list addrs on %s", ifname)
	}

	// Each object in |addrs| implements the net.Addr interface, which is not
	// very easy to use. The following code convert it to a CIDR string and then a
	// net.IP object.
	var ret IfaceAddrs
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse CIDR string %s", addr.String())
		}
		if ipv4Addr := ip.To4(); ipv4Addr != nil {
			ret.IPv4Addr = ipv4Addr
			continue
		}
		if ipv6Addr := ip.To16(); ipv6Addr != nil {
			if !ipv6Addr.IsLinkLocalUnicast() {
				ret.IPv6Addrs = append(ret.IPv6Addrs, ipv6Addr)
			}
			continue
		}
		return nil, errors.Wrapf(err, "%s is neither a v4 addr nor a v6 addr", ip.String())
	}
	return &ret, nil
}
