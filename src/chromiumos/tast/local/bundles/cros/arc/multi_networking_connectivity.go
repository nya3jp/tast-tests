// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiNetworkingConnectivity,
		Desc:         "Checks connectivity while multi-networking is enabled",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func MultiNetworkingConnectivity(ctx context.Context, s *testing.State) {
	// timeout defines the maximum allowed time for a function call.
	const timeout = 10 * time.Second

	// This code tests outbound network from within Android (ARC).
	// It fetches ARC network interfaces, and for each of the interface,
	// test if a ping to Google DNS can be made.
	ifnames, err := arc.NetworkInterfaceNames(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC interface list: ", err)
	}

	// Ensure that outbound networking works for each network interface inside ARC.
	// For multinetwork, "lo" and "arc0" are not supposed to have outbound networking
	// and as such skipped for the test.
	for _, ifname := range ifnames {
		if ifname == arc.Loopback || ifname == arc.ARC0 {
			continue
		}
		s.Log("Pinging using ", ifname)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return arc.BootstrapCommand(ctx, "/system/bin/ping", "-c1", "-w1", "-I", ifname, "8.8.8.8").Run()
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
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
	out, err := s.PreValue().(arc.PreData).ARC.Command(ctx, "/system/bin/ip", "-o", "addr", "show", "scope", "global").Output()
	if err != nil {
		s.Fatal("Failed to get android interface list: ", err)
	}

	const (
		// ip -o addr show returns network interfaces with the following format
		// space (' ') denotes whitespaces
		// "id" "interface_name" "protocol" "ip/netmask" ...
		// e.g. "103: arc0    inet 100.115.92.2/30 ..."
		// Below constants takes values from the splitted output.
		nField    = 1 // Takes interface name, e.g. "arc0"
		cidrField = 3 // Takes cidr value, e.g. "100.115.92.2/30"
		ipField   = 0 // Takes ip value from cidr, e.g. "100.115.92.2"
	)

	// Parse output of "ip -o addr show" to get interface name and ip.
	for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(s)
		if len(fields) <= cidrField {
			continue
		}
		cidr := strings.Split(fields[cidrField], "/")
		if len(cidr) <= ipField {
			continue
		}
		n := string(fields[nField])
		ifc := ifaces[n]
		ifc.arcIP = cidr[ipField]
		ifaces[n] = ifc
	}

	// Ping Android (ARC) interfaces from ChromeOS host and check for inbound traffic
	// Ensures that ARC can receive each ping and it comes from the right interface.
	s.Log("Pinging ARC interfaces")
	wg := sync.WaitGroup{}
	for ifname, ifc := range ifaces {
		if ifc.arcIP == "" || ifname == arc.ARC0 {
			continue
		}
		wg.Add(1)
		go func(ifname string, ifc iface) {
			watchCtx, watchCancel := context.WithTimeout(ctx, timeout)
			defer watchCancel()

			if err := checkNetInterface(watchCtx, ifname, ifc.arcIP, ifc.bridgeIP); err != nil {
				s.Errorf("Failed to get ping from the right interface for %s: %s", ifname, err)
			}
			wg.Done()
		}(ifname, ifc)
	}
	wg.Wait()
}

// checkNetInterface runs a ping command from ChromeOS host to Android (ARC) for a network interface
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
	defer tcpdump.Wait()
	defer tcpdump.Kill()

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
