// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package netlisten compares code shared by security.NetworkListeners tests.
package netlisten

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
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

	tastSSHPID, err := sshSessionProc(int32(os.Getpid()))
	if err != nil {
		s.Fatal("Failed to find the SSH process corresponding to the current test process: ", err)
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

		sshPID, err := sshSessionProc(st.Pid)
		if err != nil {
			sshPID = -1
		}

		// The original security_NetworkListeners Autotest test ignored any listening sockets that were owned by
		// (as reported by lsof) "autotest" commands on any address or "python" commands on 127.0.0.1. Apparently
		// Autotest also sometimes passes (or at least passed) open sockets to e.g. "sed" or "bash" child processes.
		// gopsutil doesn't appear to report duplicate connections for child processes like lsof does, so we just
		// exclude all Python and Autotest executables -- note that Python is only installed on dev and test images.
		if strings.HasPrefix(exe, "/usr/local/bin/python") || strings.HasPrefix(exe, "/usr/local/autotest/") {
			s.Logf("%v is listening at %v (probably dev- or test-related)", exe, realAddrPort)
		} else if sshPID == tastSSHPID {
			s.Logf("%v is listening at %v (Tast-related)", exe, realAddrPort)
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

// sshSessionProc returns the PID of an sshd process corresponding to the SSH
// session that a given process belongs to. It returns an error if a given
// process is not under the sshd process tree. It also returns an error if a
// given process is the root sshd process from which per-session sshd forks.
func sshSessionProc(pid int32) (int32, error) {
	for pid != 0 {
		proc, err := process.NewProcess(pid)
		if err != nil {
			return 0, err
		}
		exe, err := proc.Exe()
		if err != nil {
			return 0, err
		}
		ppid, err := proc.Ppid()
		if err != nil {
			return 0, err
		}
		if exe == "/usr/sbin/sshd" && ppid != 1 {
			return pid, nil
		}
		pid = ppid
	}
	return 0, errors.New("not an SSH session process")
}

// Common returns well-known network listeners shared between all security.NetworkListeners* tests.
func Common(cr *chrome.Chrome) map[string]string {
	return map[string]string{
		cr.DebugAddrPort(): ashproc.ExecPath,
		// p2p-http-server may be running on production systems or have been started by an earlier test.
		"*:16725": "/usr/sbin/p2p-http-server",
	}
}
