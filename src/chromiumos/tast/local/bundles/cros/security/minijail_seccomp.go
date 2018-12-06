// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MinijailSeccomp,
		Desc: "Verifies minijail0's seccomp_filter enforcement",
		Attr: []string{"informational"},
	})
}

func MinijailSeccomp(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"
		exeDir       = "/usr/libexec/security_tests/security.MinijailSeccomp"
		policyDir    = "/usr/share/security_tests/security.MinijailSeccomp"

		user = "chronos" // unprivileged user to use for testing

		okExe   = "ok"   // calls openat, read, close, exit
		openExe = "open" // calls openat (with supplied access mode), close, exit
		failExe = "fail" // calls openat, read, write, close, exit

		readOnlyArg  = "0" // O_RDONLY for openExe
		writeOnlyArg = "1" // O_WRONLY for openExe
		readWriteArg = "2" // O_RDWR for openExe

		defaultPolicy   = "policy"          // permits openat, read, close, exit, exit_group
		readOnlyPolicy  = "policy-rdonly"   // default except O_RDONLY for openat
		writeOnlyPolicy = "policy-wronly"   // default except O_WRONLY for openat
		privDropPolicy  = "policy-privdrop" // default plus setgroups, setresgid, setresuid

		exitSuccess = 0   // exit code indicating success
		exitJail    = 253 // exit code indicating a jail violation
	)

	for _, tc := range []struct {
		name       string   // test case name
		exe        string   // executable filename
		args       []string // args to pass to exe
		user       string   // user name to run as, or empty to not set user
		policy     string   // seccomp policy name to use
		noNewPrivs bool     // true to pass -n to minijail0
		exitCode   int      // expected exit code
	}{
		{"allowed syscalls", okExe, nil, "", defaultPolicy, false, exitSuccess},
		{"blocked priv-drop syscalls", okExe, nil, user, defaultPolicy, false, exitJail},
		{"no new privs", okExe, nil, user, defaultPolicy, true, exitSuccess},
		{"blocked write", failExe, nil, "", defaultPolicy, false, exitJail},
		{"allowed O_RDONLY", openExe, []string{readOnlyArg}, "", readOnlyPolicy, false, exitSuccess},
		{"blocked O_WRONLY", openExe, []string{writeOnlyArg}, "", readOnlyPolicy, false, exitJail},
		{"allowed O_WRONLY", openExe, []string{writeOnlyArg}, "", writeOnlyPolicy, false, exitSuccess},
		{"blocked O_RDWR", openExe, []string{readWriteArg}, "", writeOnlyPolicy, false, exitJail},
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
		args = append(args, "-S", filepath.Join(policyDir, tc.policy))
		args = append(args, filepath.Join(exeDir, tc.exe))
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
