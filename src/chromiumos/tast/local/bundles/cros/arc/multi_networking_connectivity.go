// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiNetworkingConnectivity,
		Desc:         "Checks connectivity while multi-networking is enabled",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

// ifnameIPRegex is a regex that extracts interface name and IP from 'ip -o addr show'.
var ifnameIPRegex = regexp.MustCompile(`^\d+:\s+\S+\s+\S+\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)

func MultiNetworkingConnectivity(ctx context.Context, s *testing.State) {
	const (
		// timeout defines the maximum allowed time for connectivity check.
		timeout = 10 * time.Second
	)

	// Hardcoded mapping of ARCVM interface name to host interface name.
	ifMap := map[string]string{
		"eth0":   "eth1",
		"eth1":   "eth2",
		"wlan0":  "eth3",
		"wlan1":  "eth4",
		"rmnet0": "eth5",
	}

	// Get the ARC instance.
	a := s.FixtValue().(*arc.PreData).ARC

	vmEnabled, err := arc.VMEnabled()
	if err != nil {
		s.Fatal("Unable to check install type of ARC: ", err)
	}

	ifnames, err := network.PhysicalInterfaces(ctx)
	if err != nil {
		s.Fatal("Failed to get physical interface list: ", err)
	}

	// Ensure that inbound and outbound networking works for each network interface inside ARC for each host's physical interfaces.
	for _, ifname := range ifnames {
		if ifup, err := interfaceUp(ctx, ifname); err != nil {
			s.Errorf("Failed checking if %s is up: %s", ifname, err)
			continue
		} else if !ifup {
			continue
		}

		arcIfname := ifname
		if vmEnabled {
			var ok bool
			arcIfname, ok = ifMap[ifname]
			if !ok {
				continue
			}
		}

		// Get the first hop from the host physical interface to Google DNS server.
		gateway, err := network.Gateway(ctx, ifname)
		if err != nil {
			s.Errorf("Failed to get gateway address for interface %s: %s", ifname, err)
			continue
		}

		// This code tests outbound network from within Android (ARC).
		// It fetches ARC network interfaces, and for each of the interface,
		// test if a ping to the gateway.
		s.Logf("Pinging from ARC interface %s through host interface %s", arcIfname, ifname)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			out, err := a.Command(ctx, "dumpsys", "wifi", "tools", "reach", arcIfname, gateway).Output()
			if err != nil {
				return err
			}
			if !strings.Contains(string(out), gateway+": reachable") {
				return errors.New("gateway unreachable")
			}
			return nil
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			s.Errorf("Failed outbound check for interface %s: %s", ifname, err)
		}

		// Get the Android (ARC) interface IP.
		out, err := a.Command(ctx, "ip", "-o", "addr", "show", arcIfname, "scope", "global").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to get Android interface list: ", err)
		}
		var arcIP string
		for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			m := ifnameIPRegex.FindStringSubmatch(o)
			if m == nil {
				continue
			}
			arcIP = m[1]
			break
		}

		// This code tests inbound network to Android (ARC).
		// It fetches ARC network interfaces, and for each of the interface,
		// test if a ping the interface is possible from the host.
		s.Logf("Pinging from host interface %s to ARC interface %s", ifname, arcIfname)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return testexec.CommandContext(ctx, "ping", "-c1", "-w1", arcIP).Run()
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			s.Errorf("Failed inbound check for interface %s: %s", ifname, err)
		}
	}
}

// interfaceUp returns true if the network interface is up.
func interfaceUp(ctx context.Context, ifname string) (bool, error) {
	out, err := testexec.CommandContext(ctx, "cat", "/sys/class/net/"+ifname+"/operstate").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "up", nil
}
