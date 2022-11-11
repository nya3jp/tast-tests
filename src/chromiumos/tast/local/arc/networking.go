// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
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

// ExpectPingSuccess checks if 'addr' is reachable over the 'network' in ARC.
// See ArcNetworkDebugTools#reachCmd for possible 'network' values.
// Use an empty 'network' to test on default network.
func ExpectPingSuccess(ctx context.Context, a *ARC, network, addr string) error {
	if network == "" {
		testing.ContextLogf(ctx, "Start to ping %s from ARC over default network", addr)
	} else {
		testing.ContextLogf(ctx, "Start to ping %s from ARC over %q", addr, network)
	}
	// This polls for 20 seconds before it gives up on pinging from within ARC. We
	// poll for a little bit since the ARP table within ARC might not be populated
	// yet - so give it some time before the ping makes it through.
	// TODO(cassiewang): We observed in the local manual tests that sometimes this
	// command gave: "*** SERVICE 'wifi' DUMP TIMEOUT (10000ms) EXPIRED ***". Need
	// to check if this also happens on the lab machines, so use a relatively
	// longer timeout here.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var cmd *testexec.Cmd
		if network == "" {
			cmd = a.Command(ctx, "dumpsys", "wifi", "tools", "reach", addr)
		} else {
			cmd = a.Command(ctx, "dumpsys", "wifi", "tools", "reach", network, addr)
		}
		if o, err := cmd.Output(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to execute 'reach' commmand, output: %s", string(o))
		} else if !strings.Contains(string(o), fmt.Sprintf("%s: reachable", addr)) {
			return errors.Errorf("ping was unreachable, output: %s", string(o))
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
		return errors.Wrap(err, "no response received in ARC")
	}

	return nil
}

// HideUnusedEthernet finds all Ethernet devices that's not being used and hide them
// from shill (thus patchpanel, and ARC) by Manager ClaimInterface API. This helps to
// workaround the current limitation of ARCVM that at most two ethernet devices can
// be supported. Returns the ifnames of devices hidden.
func HideUnusedEthernet(ctx context.Context, manager *shill.Manager) ([]string, error) {
	devices, err := manager.Devices(ctx)
	if err != nil {
		return nil, err
	}

	var toBeHidden []string
	for _, device := range devices {
		p, err := device.GetProperties(ctx)
		if err != nil {
			return nil, err
		}
		tech, err := p.GetString(shillconst.DevicePropertyType)
		if err != nil {
			return nil, err
		} else if tech != shillconst.TypeEthernet {
			continue
		}
		ips, err := p.GetObjectPaths(shillconst.DevicePropertyIPConfigs)
		if err != nil {
			return nil, err
		} else if len(ips) > 0 {
			// Do not hide device with an active IPConfig object to avoid break SSH to DUT
			continue
		}
		ifname, err := p.GetString(shillconst.DevicePropertyName)
		toBeHidden = append(toBeHidden, ifname)
	}

	for _, ifname := range toBeHidden {
		err := manager.Call(ctx, "ClaimInterface", "tast", ifname).Err
		if err != nil {
			return nil, errors.Wrapf(err, "failed to claim interface %s", ifname)
		}
	}
	return toBeHidden, nil

}

// RestoreHiddenEthernet undos the change done in HideUnusedEthernet by calling shill
// ReleaseInterface API.
func RestoreHiddenEthernet(ctx context.Context, manager *shill.Manager, ifnames []string) error {
	for _, ifname := range ifnames {
		err := manager.Call(ctx, "ReleaseInterface", "tast", ifname).Err
		if err != nil {
			return errors.Wrapf(err, "failed to release interface %s", ifname)
		}
	}
	return nil
}
