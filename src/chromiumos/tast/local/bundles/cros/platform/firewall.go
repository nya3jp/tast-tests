// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Firewall,
		Desc: "Ensure the firewall service is working correctly",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
			"jasongustaman@google.com",
			"cros-networking@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func Firewall(ctx context.Context, s *testing.State) {
	const (
		// Constants for PermissionBroker's API arguments.
		forwardPort = 1234
		accessPort  = 1235
		ip          = "100.115.92.2"
		iface       = "eth0"

		// Executables path of iptables.
		iptablesCmd  = "/sbin/iptables"
		ip6tablesCmd = "/sbin/ip6tables"

		// D-Bus constants of PermissionBroker.
		pbDbusName      = "org.chromium.PermissionBroker"
		pbDbusPath      = "/org/chromium/PermissionBroker"
		pbDbusInterface = "org.chromium.PermissionBroker"
	)

	// Connect to PermissionBroker's D-Bus service.
	_, d, err := dbusutil.Connect(ctx, pbDbusName, pbDbusPath)
	if err != nil {
		s.Fatalf("Failed to connect to D-Bus service %s: %v", pbDbusName, err)
	}

	// Call PermissionBroker's method with its arguments.
	call := func(method string, args ...interface{}) {
		result := false
		if err := d.CallWithContext(ctx, pbDbusInterface+"."+method, 0, args...).Store(&result); err != nil {
			s.Errorf("Failed to call %s: %v", method, err)
		} else if !result {
			s.Error(method + " returned false")
		}
	}

	pipe := func() (*os.File, *os.File) {
		pipeR, pipeW, err := os.Pipe()
		if err != nil {
			s.Fatal("Failed to open pipe: ", err)
		}
		return pipeR, pipeW
	}

	cleanupFds := func(pipeR, pipeW *os.File) {
		pipeR.Close()
		pipeW.Close()
	}

	// Create lifeline file descriptors and defer to cleanup.
	tcpAccessR, tcpAccessW := pipe()
	defer cleanupFds(tcpAccessR, tcpAccessW)
	tcpIfaceAccessR, tcpIfaceAccessW := pipe()
	defer cleanupFds(tcpIfaceAccessR, tcpIfaceAccessW)
	tcpForwardR, tcpForwardW := pipe()
	defer cleanupFds(tcpForwardR, tcpForwardW)
	udpAccessR, udpAccessW := pipe()
	defer cleanupFds(udpAccessR, udpAccessW)
	udpIfaceAccessR, udpIfaceAccessW := pipe()
	defer cleanupFds(udpIfaceAccessR, udpIfaceAccessW)
	udpForwardR, udpForwardW := pipe()
	defer cleanupFds(udpForwardR, udpForwardW)

	// Test case for PermissionBroker's API.
	type testCase struct {
		addMethod string        // Method to request firewall rule
		addArgs   []interface{} // Arguments for add method
		delMethod string        // Method to remove requested firewall rule
		delArgs   []interface{} // Arguments for delete method
		rule      []string      // iptables rule created
		cmds      []string      // Executable paths of firewall
	}

	var testCases = []testCase{
		testCase{
			addMethod: "RequestTcpPortAccess",
			addArgs:   []interface{}{uint16(accessPort), "", dbus.UnixFD(tcpAccessR.Fd())},
			delMethod: "ReleaseTcpPort",
			delArgs:   []interface{}{uint16(accessPort), ""},
			rule:      []string{"INPUT", "-p", "tcp", "-m", "tcp", "--dport", strconv.Itoa(accessPort), "-j", "ACCEPT", "-w"},
			cmds:      []string{iptablesCmd, ip6tablesCmd},
		},
		testCase{
			addMethod: "RequestTcpPortAccess",
			addArgs:   []interface{}{uint16(accessPort), iface, dbus.UnixFD(tcpIfaceAccessR.Fd())},
			delMethod: "ReleaseTcpPort",
			delArgs:   []interface{}{uint16(accessPort), iface},
			rule:      []string{"INPUT", "-i", iface, "-p", "tcp", "-m", "tcp", "--dport", strconv.Itoa(accessPort), "-j", "ACCEPT", "-w"},
			cmds:      []string{iptablesCmd, ip6tablesCmd},
		},
		testCase{
			addMethod: "RequestTcpPortForward",
			addArgs:   []interface{}{uint16(forwardPort), iface, ip, uint16(forwardPort), dbus.UnixFD(tcpForwardR.Fd())},
			delMethod: "ReleaseTcpPortForward",
			delArgs:   []interface{}{uint16(forwardPort), iface},
			rule:      []string{"FORWARD", "-d", ip, "-i", iface, "-p", "tcp", "-m", "tcp", "--dport", strconv.Itoa(forwardPort), "-j", "ACCEPT", "-w"},
			cmds:      []string{iptablesCmd},
		},
		testCase{
			addMethod: "RequestUdpPortAccess",
			addArgs:   []interface{}{uint16(accessPort), "", dbus.UnixFD(udpAccessR.Fd())},
			delMethod: "ReleaseUdpPort",
			delArgs:   []interface{}{uint16(accessPort), ""},
			rule:      []string{"INPUT", "-p", "udp", "-m", "udp", "--dport", strconv.Itoa(accessPort), "-j", "ACCEPT", "-w"},
			cmds:      []string{iptablesCmd, ip6tablesCmd},
		},
		testCase{
			addMethod: "RequestUdpPortAccess",
			addArgs:   []interface{}{uint16(accessPort), iface, dbus.UnixFD(udpIfaceAccessR.Fd())},
			delMethod: "ReleaseUdpPort",
			delArgs:   []interface{}{uint16(accessPort), iface},
			rule:      []string{"INPUT", "-i", iface, "-p", "udp", "-m", "udp", "--dport", strconv.Itoa(accessPort), "-j", "ACCEPT", "-w"},
			cmds:      []string{iptablesCmd, ip6tablesCmd},
		},
		testCase{
			addMethod: "RequestUdpPortForward",
			addArgs:   []interface{}{uint16(forwardPort), iface, ip, uint16(forwardPort), dbus.UnixFD(udpForwardR.Fd())},
			delMethod: "ReleaseUdpPortForward",
			delArgs:   []interface{}{uint16(forwardPort), iface},
			rule:      []string{"FORWARD", "-d", ip, "-i", iface, "-p", "udp", "-m", "udp", "--dport", strconv.Itoa(forwardPort), "-j", "ACCEPT", "-w"},
			cmds:      []string{iptablesCmd},
		},
	}

	// Delete iptables rules on exit. This allows us to clean up iptables rules unconditionally.
	defer func() {
		for _, tc := range testCases {
			for _, cmd := range tc.cmds {
				testexec.CommandContext(ctx, cmd, append([]string{"-D"}, tc.rule...)...).Run()
			}
		}
	}()

	// Call permission_broker's DBus APIs to create firewall rules.
	for _, tc := range testCases {
		call(tc.addMethod, tc.addArgs...)
	}

	// Check the result of called DBus APIs by comparing it with iptables active rules.
	for _, tc := range testCases {
		for _, cmd := range tc.cmds {
			if err := testexec.CommandContext(ctx, cmd, append([]string{"-C"}, tc.rule...)...).Run(); err != nil {
				s.Error(tc.addMethod + " failed to add " + cmd + " rule \"" + strings.Join(tc.rule, " ") + "\"")
			}
		}
	}

	// Call permission_broker's DBus APIs to delete created firewall rules.
	for _, tc := range testCases {
		call(tc.delMethod, tc.delArgs...)
	}

	// Check if the created iptables rules is successfully removed by the DBus API calls.
	for _, tc := range testCases {
		for _, cmd := range tc.cmds {
			if err := testexec.CommandContext(ctx, cmd, append([]string{"-C"}, tc.rule...)...).Run(); err == nil {
				s.Error(tc.addMethod + " failed to remove " + cmd + " rule \"" + strings.Join(tc.rule, " ") + "\"")
			}
		}
	}
}
