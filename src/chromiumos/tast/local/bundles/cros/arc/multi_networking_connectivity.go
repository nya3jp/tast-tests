// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
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
	ifnames, err := arc.GetARCInterfaceNames(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC interface list: ", err)
	}

	// Ensure that an outbound networking works for each interface
	for _, ifname := range ifnames {
		if ifname == arc.Loopback {
			continue
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return arc.BootstrapCommand(ctx, "/system/bin/ping", "-c1", "-w1", "-I", ifname, "8.8.8.8").Run()
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Errorf("Failed outbound check for interface %s: %s", ifname, err)
		}
	}

	// iface stores the IP of host bridge network interface ("arc_" prefix)
	// and IP of ARC network interface (no prefix) of a corresponding device
	// network interface (no prefix)
	type iface struct {
		bridgeIP string // Host bridge network interface IP address
		arcIP    string // ARC network interface IP address
	}

	// Get host network interfaces
	h, err := net.Interfaces()
	if err != nil {
		s.Fatal("Failed to get interface list: ", err)
	}

	// Get the bridge interfaces names and IPs (filters from host network interface)
	ifaces := make(map[string]iface)
	for _, ifc := range h {
		if strings.HasPrefix(ifc.Name, arc.BridgePrefix) {
			addrs, err := ifc.Addrs()
			if err != nil {
				s.Fatal("Failed to get interface address: ", err)
			}
			n := strings.TrimPrefix(ifc.Name, arc.BridgePrefix)
			if len(addrs) > 0 {
				ifaces[n] = iface{bridgeIP: addrs[0].(*net.IPNet).IP.String()}
			}
		}
	}

	// Get the ARC interfaces names and IPs
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := arc.BootstrapCommand(ctx, "/system/bin/ip", "-o", "addr", "show", "scope", "global").Output()
		if err != nil {
			return err
		}
		for _, s := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Fields(s)
			if len(fields) < 4 {
				continue
			}
			n := string(fields[1])
			subnet := strings.Split(fields[3], "/")
			if len(subnet) < 1 {
				continue
			}
			ip := subnet[0]
			if _, ok := ifaces[n]; !ok {
				ifaces[n] = iface{arcIP: ip}
			} else {
				ifaces[n] = iface{bridgeIP: ifaces[n].bridgeIP, arcIP: ip}
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failed to get interfaces IP address: ", err)
	}

	// Use a shortened context for test operations to reserve time for cleanup.
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	// Ping ARC interfaces from outside of ARC and check for inbound traffic
	// Ensures that ARC can receive each ping and it comes from the right interface
	for ifname, ifc := range ifaces {
		if ifc.arcIP != "" && ifname != arc.ARC0 {
			func() {
				pingCmd, err := doPing(ctx, ifc.arcIP)
				if err != nil {
					s.Fatal("Failed to start ping: ", err)
				}
				defer pingCmd.Wait()
				defer pingCmd.Kill()

				tcpdumpCmd, tcpdumpCh, err := streamTcpdump(shortCtx, ifname)
				if err != nil {
					s.Fatal("Failed to start tcpdump: ", err)
				}
				defer tcpdumpCmd.Wait()
				defer tcpdumpCmd.Kill()

				watchCtx, watchCancel := context.WithTimeout(shortCtx, 15*time.Second)
				defer watchCancel()

			WatchLoop:
				for {
					select {
					case msg := <-tcpdumpCh:
						if strings.Contains(msg, ifc.bridgeIP) {
							break WatchLoop
						}
					case <-watchCtx.Done():
						s.Errorf("Didn't see %q in tcpdump: %v", ifc.bridgeIP, watchCtx.Err())
						break WatchLoop
					}
				}
			}()
		}
	}
}

func streamTcpdump(ctx context.Context, ifname string) (*testexec.Cmd, <-chan string, error) {
	// Start a tcpdump process that writes messages to stdout on new network messages.
	cmd := arc.BootstrapCommand(ctx, "/system/xbin/tcpdump", "-i", ifname)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to start tcpdump")
	}

	ch := make(chan string)
	go func() {
		defer close(ch)

		// Writes msg to ch and returns true if more messages should be written.
		writeMsg := func(msg string) bool {
			// To avoid blocking forever on a write to ch if nobody's reading from
			// it, we use a non-blocking write. If the channel isn't writable, sleep
			// briefly and then check if the context's deadline has been reached.
			for {
				if ctx.Err() != nil {
					return false
				}

				select {
				case ch <- msg:
					return true
				default:
					testing.Sleep(ctx, 10*time.Millisecond)
				}
			}
		}

		// The Scan method will return false once the tcpdump process is killed.
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if !writeMsg(sc.Text()) {
				break
			}
		}
		// Don't bother checking sc.Err(). The test will already fail if the expected
		// message isn't seen.
	}()

	return cmd, ch, nil
}

func doPing(ctx context.Context, address string) (*testexec.Cmd, error) {
	cmd := testexec.CommandContext(ctx, "ping", address)
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start ping")
	}
	return cmd, nil
}
