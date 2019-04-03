// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AltSyscall,
		Desc: "Verifies that alt_syscall allows/blocks syscalls as expected",
		Contacts: []string{
			"jorgelo@chromium.org",  // Security team
			"ejcaruso@chromium.org", // Tast port author
			"chromeos-security@google.com",
		},
		Attr: []string{"informational"},
	})
}

func AltSyscall(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"
		// These are installed by the chromeos-base/tast-local-helpers-cros package.
		readPath         = "/usr/local/libexec/tast/helpers/local/cros/security.AltSyscall.read"
		mmapPath         = "/usr/local/libexec/tast/helpers/local/cros/security.AltSyscall.mmap"
		altSyscallPath   = "/usr/local/libexec/tast/helpers/local/cros/security.AltSyscall.alt_syscall"
		adjtimexPath     = "/usr/local/libexec/tast/helpers/local/cros/security.AltSyscall.adjtimex"
		clockAdjtimePath = "/usr/local/libexec/tast/helpers/local/cros/security.AltSyscall.clock_adjtime"
	)

	// testCase describes a minijail0 invocation.
	type testCase struct {
		name    string // human-readable test case name
		path    string // helper binary path to run (should be one of the paths above)
		table   string // alt_syscall table to use
		ret     int    // expected return code
		failMsg string // message to print if the return code is unexpected
	}

	runTestCase := func(tc testCase) {
		s.Log("Running test case ", tc.name)
		cmd := testexec.CommandContext(ctx, minijailPath, "-a", tc.table, "--", tc.path)
		ret, _ := testexec.GetWaitStatus(cmd.Run())
		if ret.ExitStatus() != tc.ret {
			s.Errorf("%v failed: %v", tc.name, tc.failMsg)
			cmd.DumpLog(ctx)
		}
	}

	for _, tc := range []testCase{
		{
			name:    "read",
			path:    readPath,
			table:   "read_write_test",
			ret:     0,
			failMsg: "Allowed system calls failed",
		},
		{
			name:    "mmap",
			path:    mmapPath,
			table:   "read_write_test",
			ret:     2,
			failMsg: "Blocked system calls allowed",
		},
		{
			name:    "alt_syscall",
			path:    altSyscallPath,
			table:   "read_write_test",
			ret:     1,
			failMsg: "Changing alt_syscall table succeeded",
		},
		{
			name:    "adjtimex",
			path:    adjtimexPath,
			table:   "android",
			ret:     0,
			failMsg: "android_adjtimex() filtering didn't work",
		},
		{
			name:    "clock_adjtime",
			path:    clockAdjtimePath,
			table:   "android",
			ret:     0,
			failMsg: "android_clock_adjtime() filtering didn't work",
		},
	} {
		runTestCase(tc)
	}
}
