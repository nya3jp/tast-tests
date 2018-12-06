// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func: MinijailSeccomp,
		Desc: "Verifies minijail0's seccomp_filter enforcement",
		Attr: []string{"informational"},
		Data: []string{
			minijailSeccompDefaultPolicy,
			minijailSeccompReadOnlyPolicy,
			minijailSeccompWriteOnlyPolicy,
			minijailSeccompPrivDropPolicy,
		},
	})
}

const (
	minijailSeccompDefaultPolicy   = "minijail_seccomp_policy"          // permits openat, read, close, exit, exit_group
	minijailSeccompReadOnlyPolicy  = "minijail_seccomp_policy-rdonly"   // default except O_RDONLY for openat
	minijailSeccompWriteOnlyPolicy = "minijail_seccomp_policy-wronly"   // default except O_WRONLY for openat
	minijailSeccompPrivDropPolicy  = "minijail_seccomp_policy-privdrop" // default plus setgroups, setresgid, setresuid
)

func MinijailSeccomp(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"

		exePrefix = "/usr/local/libexec/security_tests/security.MinijailSeccomp."
		okExe     = exePrefix + "ok"   // calls openat, read, close, exit
		openExe   = exePrefix + "open" // calls openat (with supplied access mode), close, exit
		failExe   = exePrefix + "fail" // calls openat, read, write, close, exit

		user = "chronos" // unprivileged user to use for testing

		readOnlyArg  = "0" // O_RDONLY for openExe
		writeOnlyArg = "1" // O_WRONLY for openExe
		readWriteArg = "2" // O_RDWR for openExe

		exitSuccess = 0   // exit code indicating success
		exitJail    = 253 // exit code indicating a jail violation
	)

	for _, tc := range []struct {
		name       string   // test case name
		exe        string   // executable path
		args       []string // args to pass to exe
		user       string   // user name to run as, or empty to not set user
		policy     string   // seccomp policy to use
		noNewPrivs bool     // true to pass -n to minijail0
		exitCode   int      // expected exit code
	}{
		{"allowed syscalls", okExe, nil, "", minijailSeccompDefaultPolicy, false, exitSuccess},
		{"blocked priv-drop syscalls", okExe, nil, user, minijailSeccompDefaultPolicy, false, exitJail},
		{"no new privs", okExe, nil, user, minijailSeccompDefaultPolicy, true, exitSuccess},
		{"blocked write", failExe, nil, "", minijailSeccompDefaultPolicy, false, exitJail},
		{"allowed O_RDONLY", openExe, []string{readOnlyArg}, "", minijailSeccompReadOnlyPolicy, false, exitSuccess},
		{"blocked O_WRONLY", openExe, []string{writeOnlyArg}, "", minijailSeccompReadOnlyPolicy, false, exitJail},
		{"allowed O_WRONLY", openExe, []string{writeOnlyArg}, "", minijailSeccompWriteOnlyPolicy, false, exitSuccess},
		{"blocked O_RDWR", openExe, []string{readWriteArg}, "", minijailSeccompWriteOnlyPolicy, false, exitJail},
	} {
		if ctx.Err() != nil {
			s.Error("Aborting testing: ", ctx.Err())
			break
		}

		var args []string
		if tc.user != "" {
			args = append(args, "-u", user)
		}
		if tc.noNewPrivs {
			args = append(args, "-n")
		}
		args = append(args, "-S", s.DataPath(tc.policy))
		args = append(args, tc.exe)
		args = append(args, tc.args...)

		cmd := testexec.CommandContext(ctx, minijailPath, args...)
		cmdStr := testexec.ShellEscapeArray(cmd.Args)
		s.Logf("Running %q: %v", tc.name, cmdStr)
		err := cmd.Run()

		if st, ok := testexec.GetWaitStatus(err); !ok {
			s.Errorf("Case %q (%v) failed (no exit status): %v", tc.name, cmdStr, err)
			cmd.DumpLog(ctx)
		} else if st.ExitStatus() != tc.exitCode {
			s.Errorf("Case %q (%v) exited with %d; want %d", tc.name, cmdStr, st.ExitStatus(), tc.exitCode)
			cmd.DumpLog(ctx)
		}
	}
}
