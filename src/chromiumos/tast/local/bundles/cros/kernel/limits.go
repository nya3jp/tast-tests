// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Limits,
		Desc:     "Checks kernel limits and settings in /proc",
		Contacts: []string{"chromeos-kernel@google.com"},
	})
}

func Limits(ctx context.Context, s *testing.State) {
	type op int
	const (
		eq op = iota
		ge
	)

	// Check limits stored in /proc files.
	for _, tc := range []struct {
		path string // path in /proc
		op   op     // comparison operator
		val  int64  // value to compare against
	}{
		{"/proc/sys/fs/file-max", ge, 50000}, // MemTotal-kb / 10
		{"/proc/sys/fs/leases-enable", eq, 1},
		{"/proc/sys/fs/nr_open", ge, 1048576},
		{"/proc/sys/fs/protected_hardlinks", eq, 1},
		{"/proc/sys/fs/protected_symlinks", eq, 1},
		{"/proc/sys/fs/suid_dumpable", eq, 2},
		{"/proc/sys/kernel/kptr_restrict", ge, 1},
		{"/proc/sys/kernel/ngroups_max", ge, 65536},
		{"/proc/sys/kernel/panic", eq, -1},
		{"/proc/sys/kernel/panic_on_oops", eq, 1},
		{"/proc/sys/kernel/pid_max", ge, 32768},
		{"/proc/sys/kernel/randomize_va_space", eq, 2},
		{"/proc/sys/kernel/sched_rt_period_us", eq, 1000000},
		{"/proc/sys/kernel/sched_rt_runtime_us", eq, 800000},
		{"/proc/sys/kernel/sysrq", eq, 1},
		{"/proc/sys/kernel/threads-max", ge, 7000}, // MemTotal-kb / 64
		{"/proc/sys/kernel/yama/ptrace_scope", eq, 1},
		{"/proc/sys/net/ipv4/tcp_syncookies", eq, 1},
		{"/proc/sys/vm/mmap_min_addr", ge, 32768},
	} {
		b, err := ioutil.ReadFile(tc.path)
		if err != nil {
			s.Errorf("Failed to read %v: %v", tc.path, err)
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			s.Errorf("Failed to parse %q from %v: %v", b, tc.path, err)
			continue
		}

		if tc.op == eq && val != tc.val {
			s.Errorf("%v contains %v; want %v", tc.path, val, tc.val)
		} else if tc.op == ge && val < tc.val {
			s.Errorf("%v contains %v; want at least %v", tc.path, val, tc.val)
		} else {
			s.Logf("%v contains %v", tc.path, val)
		}
	}

	// Check rlimits (rather than parsing /proc/self/limits).
	for _, tc := range []struct {
		name string // human-readable name
		res  int    // resource ID
		min  uint64 // minimum acceptable value
	}{
		{"RLIMIT_NOFILE", unix.RLIMIT_NOFILE, 1024},
		{"RLIMIT_NPROC", unix.RLIMIT_NPROC, 3000}, // MemTotal-kb / 128
	} {
		var rlimit unix.Rlimit
		if err := unix.Getrlimit(tc.res, &rlimit); err != nil {
			s.Errorf("Failed to get %v: %v", tc.name, err)
		} else if rlimit.Cur < tc.min {
			s.Errorf("%v is %v; want at least %v", tc.name, rlimit.Cur, tc.min)
		} else {
			s.Logf("%v is %v", tc.name, rlimit.Cur)
		}
	}
}
