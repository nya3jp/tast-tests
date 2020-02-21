// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
)

const (
	// VethPrefix is a prefix for host veth interfaces name.
	VethPrefix = "veth_"
	// BridgePrefix is a prefix for host bridge interfaces name.
	BridgePrefix = "arc_"
	// VmtapPrefix is a prefix for host vmtap interfaces name.
	VmtapPrefix = "vmtap"

	// ARCBR0 refers to a host network bridge interface named arcbr0.
	ARCBR0 = "arcbr0"
	// ARC0 refers to an ARC network interface named arc0.
	ARC0 = "arc0"
	// ARC1 refers to an ARC network interface named arc1 which is created for ARCVM.
	// On dual-boards, ARC++ might have this interface as a result of switching ARCVM -> ARC++.
	// ARC++ should not care about this interface.
	ARC1 = "arc1"
	// Loopback refers to loopback interface named lo.
	Loopback = "lo"

	// Android interface prefixes
	clatPrefix = "v4-"
	vpnPrefix  = "tun"
)

// NetworkInterfaceNames filters Android interfaces and returns ARC related network interfaces.
func NetworkInterfaceNames(ctx context.Context) ([]string, error) {
	out, err := BootstrapCommand(ctx, "/system/bin/ls", "/sys/class/net/").Output()
	if err != nil {
		return nil, err
	}

	// Filter out non-arc android net interfaces
	var ifnames []string
	for _, ifname := range strings.Fields(string(out)) {
		if !strings.HasPrefix(ifname, clatPrefix) &&
			!strings.HasPrefix(ifname, vpnPrefix) &&
			ifname != Loopback {
			ifnames = append(ifnames, ifname)
		}
	}

	return ifnames, nil
}
