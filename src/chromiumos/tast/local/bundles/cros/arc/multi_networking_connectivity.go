// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"net"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiNetworkingConnectivity,
		Desc:         "Checks connectivity while multi-networking is enabled",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Pre:          arc.Booted(),
	})
}

// ifnameIPRegex is a regex that extracts interface name and IP from 'ip -o addr show'.
var ifnameIPRegex = regexp.MustCompile(`^\d+:\s+(\S+)\s+\S+\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)

func MultiNetworkingConnectivity(ctx context.Context, s *testing.State) {
	const (
		// outTimeout defines the maximum allowed time for outbound connectivity check.
		outTimeout = 10 * time.Second

		// inTimeout defines the maximum allowed time for inbound connectivity check.
		inTimeout = 30 * time.Second
	)

	// This code tests outbound network from within Android (ARC).
	// It fetches ARC network interfaces, and for each of the interface,
	// test if a ping to Google DNS can be made.
	ifnames, err := arc.NetworkInterfaceNames(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC interface list: ", err)
	}

	// Ensure that outbound networking works for each network interface inside ARC.
	// We are skipping interfaces:
	// - that is guest-only ("arc0" and "arc1"),
	// - without outbound networking ("lo"),
	// - that is not up.
	for _, ifname := range ifnames {
		if ifname == arc.Loopback || ifname == arc.ARC0 || ifname == arc.ARC1 {
			continue
		}

		if ifup, err := interfaceUp(ifname); err != nil {
			s.Errorf("Failed checking if %s is up: %s", ifname, err)
			continue
		} else if !ifup {
			continue
		}

		gateway, err := network.Gateway(ctx, ifname)
		if err != nil {
			s.Errorf("Failed to get gateway address for interface %s: %s", ifname, err)
			continue
		}

		s.Log("Pinging using ", ifname)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return arc.BootstrapCommand(ctx, "/system/bin/ping", "-c1", "-w1", "-I", ifname, gateway).Run()
		}, &testing.PollOptions{Timeout: outTimeout}); err != nil {
			s.Errorf("Failed outbound check for interface %s: %s", ifname, err)
		}
	}

	// This code tests inbound network of Android (ARC). It checks if a ping from
	// Chrome OS host can be made to each corresponding network interface of ARC.
	// Currently, it does not check if ARC inbound network works from outside of
	// the host.

	// iface stores the IP of host bridge network interface ("arc_" prefix)
	// and IP of ARC network interface (no prefix) of a corresponding device
	// network interface (no prefix)
	type iface struct {
		bridgeIP string // Host bridge network interface IP address
		arcIP    string // ARC network interface IP address
	}

	// Get host network interfaces.
	h, err := net.Interfaces()
	if err != nil {
		s.Fatal("Failed to get host interface list: ", err)
	}

	// Get the bridge interfaces names and IPs (filters from host network interface).
	ifaces := make(map[string]iface)
	for _, ifc := range h {
		n := strings.TrimPrefix(ifc.Name, arc.BridgePrefix)
		if n == ifc.Name {
			continue
		}
		if addrs, err := ifc.Addrs(); err != nil {
			s.Errorf("Failed to get %s interface address: %s", ifc.Name, err)
		} else if len(addrs) > 0 {
			ifaces[n] = iface{bridgeIP: addrs[0].(*net.IPNet).IP.String()}
		}
	}

	// Get the Android (ARC) interfaces names and IPs.
	out, err := s.PreValue().(arc.PreData).ARC.Command(ctx, "/system/bin/ip", "-o", "addr", "show", "scope", "global").Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to get Android interface list: ", err)
	}

	// ip -o addr show returns network interfaces with the following format
	// space (' ') denotes whitespaces
	// "id" "interface_name" "protocol" "ip/netmask" ...
	// e.g. "103: arc0    inet 100.115.92.2/30 ..."
	// Parse output of "ip -o addr show" to get interface name and ip.
	for _, o := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		m := ifnameIPRegex.FindStringSubmatch(o)
		if m == nil {
			continue
		}
		name := m[1]
		ifc := ifaces[name]
		ifc.arcIP = m[2]
		ifaces[name] = ifc
	}

	// Ping Android (ARC) interfaces from Chrome OS host and check for inbound traffic
	// Ensures that ARC can receive each ping and it comes from the right interface.
	s.Log("Pinging ARC interfaces")

	// Create a shorter context for inbound traffic check.
	watchCtx, watchCancel := context.WithTimeout(ctx, inTimeout)
	defer watchCancel()

	g, watchCtx := errgroup.WithContext(watchCtx)
	for ifname, ifc := range ifaces {
		if ifc.arcIP == "" || ifname == arc.ARC0 || ifname == arc.ARC1 {
			continue
		}

		if ifup, err := interfaceUp(ifname); err != nil {
			s.Errorf("Failed checking if %s is up: %s", ifname, err)
			continue
		} else if !ifup {
			continue
		}

		ifname, ifc := ifname, ifc // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			if err := checkNetInterface(watchCtx, ifname, ifc.arcIP, ifc.bridgeIP); err != nil {
				return errors.Errorf("failed to get ping from the right interface for %s, %s", ifname, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		s.Fatal("Failed inbound traffic check: ", err)
	}
}

// checkNetInterface runs a ping command from Chrome OS host to Android (ARC) for a network interface
// "ifname" and checks if the right ping went through by checking the IP.
// It ensures that the ARC and bridge network interface is created properly.
func checkNetInterface(ctx context.Context, ifname, arcIP, bridgeIP string) error {
	// Starts a ping command to "arcIP".
	ping := testexec.CommandContext(ctx, "ping", "-i0.3", arcIP)
	if err := ping.Start(); err != nil {
		return errors.Wrap(err, "failed to start ping")
	}
	defer ping.Wait()
	defer ping.Kill()

	// Starts a tcpdump process that writes messages to stdout on new network messages.
	tcpdump := arc.BootstrapCommand(ctx, "/system/xbin/tcpdump", "-i", ifname)

	stdout, err := tcpdump.StdoutPipe()
	if err != nil {
		return err
	}

	if err := tcpdump.Start(); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}

	// sc.Scan() below might block. Release bufio.Scanner by killing tcpdump if the
	// process execution time exceeds context deadline.
	go func() {
		defer tcpdump.Wait()
		defer tcpdump.Kill()

		// Blocks until deadline is passed.
		<-ctx.Done()
	}()

	// Watch and wait until tcpdump output "bridgeIP".
	sc := bufio.NewScanner(stdout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sc.Scan() && strings.Contains(sc.Text(), bridgeIP) {
			return nil
		}
	}
}

// interfaceUp returns true if the network interface is up.
func interfaceUp(ifname string) (bool, error) {
	iface, err := net.InterfaceByName(ifname)
	if err != nil {
		return false, err
	}
	return strings.Contains(iface.Flags.String(), "up"), nil
}
