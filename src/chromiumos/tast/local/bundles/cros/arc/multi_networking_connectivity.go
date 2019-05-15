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
		Desc:         "Checks that connectivity while multi-networking works as expected",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func MultiNetworkingConnectivity(ctx context.Context, s *testing.State) {
	const (
		// timeout defines the maximum allowed time for a function call.
		timeout = 10 * time.Second

		// ip addr -o show returns network interfaces with the following format
		// space (' ') denotes whitespaces
		// "id" "interface_name" "protocol" "ip/netmask" ...
		// e.g. "103: arc0    inet 100.115.92.2/30 ..."
		// Below constants takes values from the splitted output.
		nField    = 1 // Takes interface name, e.g. "arc0"
		cidrField = 3 // Takes cidr value, e.g. "100.115.92.2/30"
		ipField   = 0 // Takes ip value from cidr, e.g. "100.115.92.2"
	)

	// This chunks of code test outbound network from within Android (ARC).
	// It fetches ARC network interfaces, and for each of the interface,
	// test if a ping to Google DNS can be made.

	// Retrieve Android (ARC) network interfaces.
	ifnames, err := arc.GetARCInterfaceNames(ctx)
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
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return s.PreValue().(arc.PreData).ARC.Command(ctx, "/system/bin/ping", "-c1", "-w1", "-I", ifname, "8.8.8.8").Run()
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			s.Errorf("Failed outbound check for interface %s: %s", ifname, err)
		}
	}

	// This chunks of code test inbound network of Android (ARC). It checks if
	// a ping from ChromeOS host can be made to each corresponding network interface
	// of ARC. Currently, it does not check if ARC inbound network of works from
	// outside of the host.

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
		s.Fatal("Failed to get interface list: ", err)
	}

	// Get the bridge interfaces names and IPs (filters from host network interface).
	ifaces := make(map[string]iface)
	for _, ifc := range h {
		n := strings.TrimPrefix(ifc.Name, arc.BridgePrefix)
		if n == ifc.Name {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			s.Errorf("Failed to get %s interface address: %s", ifc.Name, err)
		}
		if len(addrs) > 0 {
			ifaces[n] = iface{bridgeIP: addrs[0].(*net.IPNet).IP.String()}
		}

	}

	// Get the Android (ARC) interfaces names and IPs.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := s.PreValue().(arc.PreData).ARC.Command(ctx, "/system/bin/ip", "-o", "addr", "show", "scope", "global").Output()
		if err != nil {
			return err
		}
		for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Fields(s)
			if len(fields) <= cidrField {
				continue
			}
			cidr := strings.Split(fields[cidrField], "/")
			if len(cidr) <= ipField {
				continue
			}
			ip := cidr[ipField]
			n := string(fields[nField])
			if _, ok := ifaces[n]; !ok {
				ifaces[n] = iface{arcIP: ip}
			} else {
				ifaces[n] = iface{bridgeIP: ifaces[n].bridgeIP, arcIP: ip}
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		s.Error("Failed to get interfaces IP address: ", err)
	}

	// Ping Android (ARC) interfaces from ChromeOS host and check for inbound traffic
	// Ensures that ARC can receive each ping and it comes from the right interface.
	wg := sync.WaitGroup{}
	for ifname, ifc := range ifaces {
		if ifc.arcIP == "" || ifname == arc.ARC0 {
			continue
		}
		wg.Add(1)
		go func(ifname string, ifc iface) {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				return checkNetInterface(ctx, s.PreValue().(arc.PreData).ARC, ifname, ifc.arcIP, ifc.bridgeIP)
			}, &testing.PollOptions{Timeout: timeout}); err != nil {
				s.Error("Failed to get ping from the right interface: ", err)
			}
			wg.Done()
		}(ifname, ifc)
	}
	wg.Wait()
}

// checkNetInterface runs a ping command from ChromeOS host to Android (ARC) for a network interface
// "ifname" and checks if the right ping went through by checking the IP.
// It ensures that the ARC and bridge network interface is created properly.
func checkNetInterface(ctx context.Context, a *arc.ARC, ifname string, arcIP string, bridgeIP string) error {
	pingCmd, err := doPing(ctx, arcIP)
	if err != nil {
		return err
	}
	defer pingCmd.Wait()
	defer pingCmd.Kill()

	err = streamTcpdump(ctx, a, ifname, func(msg string) bool {
		if !strings.Contains(msg, bridgeIP) {
			return false
		}
		return true
	})
	if err != nil {
		return errors.Wrapf(err, "failed to get ping for interface %s", ifname)
	}
	return nil
}

// streamTcpdump starts a tcpdump process and wait until an exit condition is fulfilled from
// "out" function. The function also checks if it exceeds the time constraint in "ctx".
func streamTcpdump(ctx context.Context, a *arc.ARC, ifname string, out func(string) bool) error {
	// Start a tcpdump process that writes messages to stdout on new network messages.
	cmd := a.Command(ctx, "/system/xbin/tcpdump", "-i", ifname)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start tcpdump")
	}
	defer cmd.Wait()
	defer cmd.Kill()

	sc := bufio.NewScanner(stdout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if sc.Scan() && !out(sc.Text()) {
			return nil
		}
	}
}

// doPing start a ping command to address "address" and returns the command "cmd"
func doPing(ctx context.Context, address string) (*testexec.Cmd, error) {
	cmd := testexec.CommandContext(ctx, "ping", address)
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start ping")
	}
	return cmd, nil
}
