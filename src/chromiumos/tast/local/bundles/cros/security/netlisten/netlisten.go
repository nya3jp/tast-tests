// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netlisten compares code shared by security.NetworkListeners tests.
package netlisten

import (
	"context"
	"fmt"
	"strings"

	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// CheckPorts compares processes listening for TCP connections against an expected set.
// The keys in expected take the form "<addr>:<port>", with "*" used to indicate "any address"
// for both IPv4 and IPv6 (e.g. "*:22" or "127.0.0.1:80"). The values are absolute paths to
// the executables that should be listening on the ports (e.g. "/usr/sbin/sshd").
// Testing-related processes are automatically excluded.
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

		exe, err := getExe(st.Pid)
		if err != nil {
			s.Logf("Process %d was listening at %v (probably exited)", st.Pid, realAddrPort)
			continue
		}

		// The original security_NetworkListeners Autotest test ignored any listening sockets that were owned by
		// (as reported by lsof) "autotest" commands on any address or "python" commands on 127.0.0.1. Apparently
		// Autotest also sometimes passes (or at least passed) open sockets to e.g. "sed" or "bash" child processes.
		// gopsutil doesn't appear to report duplicate connections for child processes like lsof does, so we just
		// exclude all Python and Autotest executables -- note that Python is only installed on dev and test images.
		if strings.HasPrefix(exe, "/usr/local/bin/python") || strings.HasPrefix(exe, "/usr/local/autotest/") {
			s.Logf("%v is listening at %v (probably dev- or test-related)", exe, realAddrPort)
		} else if expExe, expOpen := expected[addrPort]; !expOpen {
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

// Common returns well-known network listeners shared between all security.NetworkListeners* tests.
func Common(cr *chrome.Chrome) (map[string]string, error) {
	addrport, err := cr.DebugAddrPort()
	if err != nil {
		return map[string]string{}, err
	}
	return map[string]string{
		addrport: chrome.ExecPath,
		// p2p-http-server may be running on production systems or have been started by an earlier test.
		"*:16725": "/usr/sbin/p2p-http-server",
		// Tast may forward port 28082 to the ephemeral devserver.
		"127.0.0.1:28082": "/usr/sbin/sshd",
	}, nil
}
