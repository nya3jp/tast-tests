// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"regexp"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// ipv4Regex is a regex that matches IPv4 address.
var ipv4Regex = regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)

// Gateway returns IPv4 gateway address of an interface by calling "ip route get 8.8.8.8".
func Gateway(ctx context.Context, ifname string) (string, error) {
	// Get the physical interface gateway address.
	out, err := testexec.CommandContext(ctx, "/bin/ip", "route", "get", "8.8.8.8", "oif", ifname).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get gateway address for interface %s", ifname)
	}

	// ip route get 8.8.8.8 returns address of the next hop to 8.8.8.8 with the following format
	// 8.8.8.8 via "gateway_address" ...
	// e.g. "8.8.8.8 via 100.87.84.254 ..."
	// Gateway is the second valid IPv4 address.
	m := ipv4Regex.FindAllString(string(out), 2)
	if len(m) < 2 {
		return "", errors.Errorf("failed to parse gateway address from 'ip route' for interface %s", ifname)
	}
	gateway := m[1]
	return gateway, nil
}
