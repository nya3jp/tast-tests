// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ArcNetworkdUp,
		Desc: "Checks that both arc-networkd processes are up",
		Attr: []string{"informational"},
	})
}

type info struct {
	pid  int32
	ppid int32
	args string
	user string
}

func ArcNetworkdUp(ctx context.Context, s *testing.State) {
	const (
		arcUser         = "arc-networkd"
		binPath         = "/usr/bin/arc-networkd"
		helperArgPrefix = "--ip_helper_fd="
	)

	all, err := process.Processes()
	if err != nil {
		s.Fatal("Failed to get process list: ", err)
	}

	var pi []info
	for _, proc := range all {
		if exe, _ := proc.Exe(); exe == binPath {
			ppid, err := proc.Ppid()
			if err != nil {
				s.Fatal("Could not obtain parent of PID: ", proc.Pid)
			}
			args, err := proc.Cmdline()
			if err != nil {
				s.Fatal("Could not obtain args for PID: ", proc.Pid)
			}
			u, err := proc.Username()
			if err != nil {
				s.Fatal("Could not obtain username for PID: ", proc.Pid)
			}
			pi = append(pi, info{proc.Pid, ppid, args, u})
		}
	}
	if len(pi) != 2 {
		s.Fatalf("Unexpected number of processes; got %d, wanted 2", len(pi))
	}
	for i, a := range pi {
		b := pi[(i+1)%2]
		if a.ppid == 1 {
			if a.args != binPath {
				s.Fatal("Manager process has run with unexpected args: ", a.args)
			}
			if a.user != arcUser {
				s.Fatal("Manager process running as unexpected user: ", a.user)
			}
		} else {
			if b.ppid != 1 {
				s.Fatal("Manager process has unexpected parent PID: ", b.ppid)
			}
			if a.ppid != b.pid {
				s.Fatal("Helper process has unexpected parent PID: ", a.ppid)
			}
			args := strings.Split(a.args, " ")
			if len(args) != 2 {
				s.Fatal("Helper process has unexpected args: ", a.args)
			}
			if _, err := strconv.Atoi(strings.TrimPrefix(args[1], helperArgPrefix)); err != nil {
				s.Fatal("Helper process has unexpected args (not a file descriptor):", args[0])
			}
		}
	}
}
