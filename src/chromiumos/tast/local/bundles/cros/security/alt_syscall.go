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
		SoftwareDeps: []string{"alt_syscall"},
		Attr:         []string{"informational"},
	})
}

func AltSyscall(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"
		// These are installed by the chromeos-base/tast-local-helpers-cros package.
		basePath         = "/usr/local/libexec/tast/helpers/local/cros/security.AltSyscall."
		readPath         = basePath + "read"
		mmapPath         = basePath + "mmap"
		altSyscallPath   = basePath + "alt_syscall"
		adjtimexPath     = basePath + "adjtimex"
		clockAdjtimePath = basePath + "clock_adjtime"
	)

	for _, tc := range []struct {
		name    string // human-readable test case name
		path    string // helper binary path to run (should be one of the paths above)
		table   string // alt_syscall table to use
		ret     int    // expected return code
		failMsg string // message to print if the return code is unexpected
	}{
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
		s.Log("Running test case ", tc.name)
		cmd := testexec.CommandContext(ctx, minijailPath, "-a", tc.table, "--", tc.path)
		ws, _ := testexec.GetWaitStatus(cmd.Run())
		ret := ws.ExitStatus()
		if ret != tc.ret {
			s.Errorf("%v failed: %v (wanted exit status %d, got %d)",
				tc.name, tc.failMsg, tc.ret, ret)
			cmd.DumpLog(ctx)
		}
	}
}
