// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package virtualnet

import (
	"fmt"
	"net"

	"chromiumos/tast/errors"
)

type subnetPool struct {
	ipv4Next int
	ipv6Next int
}

const (
	subnetStart = 100
	subnetEnd   = 200
)

// NewSubnetPool creates a new pool for IP subnets. The same pool should be used
// in one test to avoid address collisions.
func NewSubnetPool() *subnetPool {
	return &subnetPool{ipv4Next: subnetStart, ipv6Next: subnetStart}
}

// AllocNextIPv4Subnet allocates the next IPv4 subnet.
func (p *subnetPool) AllocNextIPv4Subnet() (*net.IPNet, error) {
	return allocNextSubnet(&p.ipv4Next, "192.168.%d.1/24")
}

// AllocNextIPv4Subnet allocates the next IPv6 subnet.
func (p *subnetPool) AllocNextIPv6Subnet() (*net.IPNet, error) {
	return allocNextSubnet(&p.ipv6Next, "fc%02x::1/64")
}

func allocNextSubnet(next *int, cidrFmt string) (*net.IPNet, error) {
	if *next > subnetEnd {
		return nil, errors.New("no available subnet")
	}
	cidr := fmt.Sprintf(cidrFmt, *next)
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse CIDR %s", cidr)
	}
	*next++
	return subnet, nil
}
