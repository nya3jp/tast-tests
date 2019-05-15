// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package arc supports interacting with the ARC framework, which is used to run Android applications on Chrome OS.
package arc

import (
	"context"
	"strings"
)

const (
	// VethPrefix is a prefix for host veth interfaces name
	VethPrefix = "veth_"
	// BridgePrefix is a prefix for host bridge interfaces name
	BridgePrefix = "arc_"
	// VmtapPrefix is a prefix for host vmtap interfaces name
	VmtapPrefix = "vmtap"

	// Arcbr0 interface
	Arcbr0 = "arcbr0"
	// Arc0 interface
	Arc0 = "arc0"
	// Loopback interface
	Loopback = "lo"

	// Android interface prefixes
	clatPrefix = "v4-"
	vpnPrefix  = "tun"
)

// GetARCInterfaceNames filters Android interfaces and returns ARC related network interfaces.
func GetARCInterfaceNames(ctx context.Context) ([]string, error) {
	out, err := BootstrapCommand(ctx, "/system/bin/ls", "/sys/class/net/").Output()
	if err != nil {
		return nil, err
	}
	val := strings.Split(strings.TrimSpace(string(out)), "\n")

	// Filter out non-arc android net interfaces
	ifnames := make([]string, 0)
	for _, ifname := range val {
		if !strings.HasPrefix(ifname, clatPrefix) &&
			!strings.HasPrefix(ifname, vpnPrefix) &&
			strings.Compare(ifname, Loopback) != 0 {
			ifnames = append(ifnames, ifname)
		}
	}

	return ifnames, nil
}
