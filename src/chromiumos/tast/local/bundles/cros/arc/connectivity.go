// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Connectivity,
		Desc:         "Checks that connectivity while multi-networking works as expected",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func Connectivity(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	outboundNetworking(ctx, s)
	inboundNetworking(ctx, s)
}

func outboundNetworking(ctx context.Context, s *testing.State) {
	ifnames, err := arc.GetARCInterfaceNames(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC interface list: ", err)
	}

	// Ensure that an outbound networking works for each interface
	for _, ifname := range ifnames {
		if strings.Compare(ifname, arc.Loopback) == 0 {
			continue
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := arc.BootstrapCommand(ctx, "/system/bin/ping", "-c1", "-w1", "-I", ifname, "8.8.8.8").Run(); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 2 * time.Second}); err != nil {
			s.Errorf("Failed outbound check for interface %s: %s", ifname, err)
		}
	}
}

func inboundNetworking(ctx context.Context, s *testing.State) {
	netIfaces, err := net.Interfaces()
	if err != nil {
		s.Fatal("Failed to get interface list: ", err)
	}

	type iface struct {
		bridge string
		arc    string
	}

	// Get the bridge interfaces names and IPs
	ifaces := make(map[string]*iface)
	for _, i := range netIfaces {
		if strings.HasPrefix(i.Name, arc.BridgePrefix) {
			addrs, err := i.Addrs()
			if err != nil {
				s.Fatal("Failed to get interface address: ", err)
			}
			n := strings.TrimPrefix(i.Name, arc.BridgePrefix)
			ifaces[n] = new(iface)
			if len(addrs) > 0 {
				ifaces[n].bridge = addrs[0].(*net.IPNet).IP.String()
			} else {
				ifaces[n].bridge = ""
			}
		}
	}

	// Get the ARC interfaces names and IPs
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := arc.BootstrapCommand(ctx, "/system/bin/ip", "-o", "addr", "show", "scope", "global").Output()
		if err != nil {
			return err
		}
		a := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, i := range a {
			fields := strings.Fields(i)
			n := string(fields[1])
			ip := strings.Split(fields[3], "/")[0]
			if _, ok := ifaces[n]; !ok {
				ifaces[n] = new(iface)
			}
			ifaces[n].arc = ip
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second}); err != nil {
		s.Error("Failed to get interfaces IP address: ", err)
	}

	// Ping ARC interfaces from outside of ARC and check for inbound traffic
	// Ensures that ARC can receive each connection from the right interface
	for ifname, ip := range ifaces {
		if strings.Compare(ip.arc, "") != 0 && strings.Compare(ifname, arc.Arc0) != 0 {
			go doPing(ctx, s, ip.arc)
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				out, err := arc.BootstrapCommand(ctx, "/vendor/bin/timeout", "2", "/system/xbin/tcpdump", "-i", ifname).Output()
				if err != nil {
					return err
				}
				if !strings.Contains(string(out), ip.bridge) {
					return errors.Errorf("No inbound connection from %s(%s) to %s(%s)", arc.BridgePrefix+ifname, ip.bridge, ifname, ip.arc)
				}
				return nil
			}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
				s.Error("Failed to get inbound connection: ", err)
			}
		}
	}
}

func doPing(ctx context.Context, s *testing.State, address string) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "ping", "-i0.3", "-w2", address).Run(); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		s.Error("Ping cannot reach target: ", err)
	}
}
