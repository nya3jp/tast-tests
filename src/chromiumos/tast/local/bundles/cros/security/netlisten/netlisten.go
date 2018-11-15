// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netlisten compares code shared by security.NetworkListeners tests.
package netlisten

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// CheckPorts compares processes listening for TCP connections against an expected set.
// The keys in expected take the form "<addr>:<port>", with "*" used to indicate "any address"
// for both IPv4 and IPv6 (e.g. "*:22" or "127.0.0.1:80"). The values are absolute paths to
// the executables that should be listening on the ports (e.g. "/usr/sbin/sshd").
func CheckPorts(ctx context.Context, s *testing.State, expected map[string]string) {
	stats, err := net.Connections("tcp")
	if err != nil {
		s.Fatal("Failed to list connections: ", err)
	}
	for _, st := range stats {
		if st.Status != "LISTEN" {
			continue
		}

		// Use a protocol-agnostic form for comparing against "any address", but log the actual address.
		addr := st.Laddr.IP
		if addr == "0.0.0.0" || addr == "::" {
			addr = "*"
		}
		addrPort := fmt.Sprintf("%s:%d", addr, st.Laddr.Port)
		realAddrPort := fmt.Sprintf("%s:%d", st.Laddr.IP, st.Laddr.Port)

		expExe, expOpen := expected[addrPort]

		exe, err := getExe(st.Pid)
		if err != nil {
			// Assume that the process went away, but still check that the port was expected.
			if !expOpen {
				s.Error("Exited process was listening at ", realAddrPort)
			}
			continue
		}

		if !expOpen {
			s.Errorf("%v is listening at %v", exe, realAddrPort)
		} else if exe != expExe {
			s.Errorf("%v is listening at %v; want %v", exe, realAddrPort, expExe)
		} else {
			s.Logf("%v is listening at %v", exe, realAddrPort)
		}
	}
}

// getExe returns the executable path corresponding to the supplied PID.
// An error may be returned if the process exits before it is examined.
func getExe(pid int32) (string, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return "", err
	}
	return proc.Exe()
}

// SSHListeners returns standard expected listeners (in the format expected by CheckPorts) for SSH connections.
// The result may differ depending on whether the DUT supports ARC or not.
func SSHListeners(ctx context.Context) map[string]string {
	const (
		sshdExe = "/usr/sbin/sshd"
		sslhExe = "/usr/sbin/sslh-fork"
	)

	// sslh is installed on ARC-capable systems to multiplex port 22 traffic between sshd and adb.
	if upstart.JobExists(ctx, "sslh") {
		return map[string]string{
			"*:2222": sshdExe,
			"*:22":   sslhExe,
		}
	}
	return map[string]string{"*:22": sshdExe}
}

// ChromeListeners returns standard expected listeners (in the format expected by CheckPorts) for Chrome.
func ChromeListeners(ctx context.Context, cr *chrome.Chrome) map[string]string {
	return map[string]string{cr.DebugAddrPort(): "/opt/google/chrome/chrome"}
}
