// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MinijailSeccomp,
		Desc: "Verifies minijail0's seccomp_filter enforcement",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		Data: []string{
			minijailSeccompDefaultPolicy,
			minijailSeccompReadOnlyPolicy,
			minijailSeccompWriteOnlyPolicy,
			minijailSeccompPrivDrop32Policy,
			minijailSeccompPrivDrop64Policy,
		},
	})
}

const (
	minijailSeccompDefaultPolicy    = "minijail_seccomp_policy"             // permits openat, read, close, exit, exit_group
	minijailSeccompReadOnlyPolicy   = "minijail_seccomp_policy-rdonly"      // default except O_RDONLY for openat
	minijailSeccompWriteOnlyPolicy  = "minijail_seccomp_policy-wronly"      // default except O_WRONLY for openat
	minijailSeccompPrivDrop32Policy = "minijail_seccomp_policy-privdrop_32" // default plus setgroups, setresgid, setresuid
	minijailSeccompPrivDrop64Policy = "minijail_seccomp_policy-privdrop_64" // 64-bit version of privdrop
)

func MinijailSeccomp(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"

		// These binaries are installed by the chromeos-base/tast-local-helpers-cros package.
		exePrefix = "/usr/local/libexec/tast/helpers/local/cros/security.MinijailSeccomp."
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

	// Choose the correct version of the priv-drop policy depending on the userspace arch.
	var privDropPolicy string
	ptrSize := 32 << uintptr(^uintptr(0)>>63) // from https://stackoverflow.com/questions/25741841/
	switch ptrSize {
	case 32:
		privDropPolicy = minijailSeccompPrivDrop32Policy
	case 64:
		privDropPolicy = minijailSeccompPrivDrop64Policy
	default:
		s.Fatal("Unexpected pointer size ", ptrSize)
	}

	// Controls whether -n is passed to minijail0 to set no_new_privs:
	// https://www.kernel.org/doc/Documentation/prctl/no_new_privs.txt
	type newPrivsState int
	const (
		allowNewPrivs newPrivsState = iota
		noNewPrivs
	)

	for _, tc := range []struct {
		name     string   // test case name
		exe      string   // executable path
		args     []string // args to pass to exe
		user     string   // user name to run as, or empty to not set user
		policy   string   // seccomp policy to use
		newPrivs newPrivsState
		exitCode int // expected exit code
	}{
		{"allowed syscalls", okExe, nil, "", minijailSeccompDefaultPolicy, allowNewPrivs, exitSuccess},
		{"blocked priv-drop syscalls", okExe, nil, user, minijailSeccompDefaultPolicy, allowNewPrivs, exitJail},
		{"allowed priv-drop syscalls", okExe, nil, user, privDropPolicy, allowNewPrivs, exitSuccess},
		{"no_new_privs", okExe, nil, user, minijailSeccompDefaultPolicy, noNewPrivs, exitSuccess},
		{"blocked write", failExe, nil, "", minijailSeccompDefaultPolicy, allowNewPrivs, exitJail},
		{"allowed O_RDONLY", openExe, []string{readOnlyArg}, "", minijailSeccompReadOnlyPolicy, allowNewPrivs, exitSuccess},
		{"blocked O_WRONLY", openExe, []string{writeOnlyArg}, "", minijailSeccompReadOnlyPolicy, allowNewPrivs, exitJail},
		{"allowed O_WRONLY", openExe, []string{writeOnlyArg}, "", minijailSeccompWriteOnlyPolicy, allowNewPrivs, exitSuccess},
		{"blocked O_RDWR", openExe, []string{readWriteArg}, "", minijailSeccompWriteOnlyPolicy, allowNewPrivs, exitJail},
	} {
		if ctx.Err() != nil {
			s.Error("Aborting testing: ", ctx.Err())
			break
		}

		var args []string
		if tc.user != "" {
			args = append(args, "-u", user)
		}
		if tc.newPrivs == noNewPrivs {
			args = append(args, "-n")
		}
		args = append(args, "-S", s.DataPath(tc.policy))
		args = append(args, tc.exe)
		args = append(args, tc.args...)

		cmd := testexec.CommandContext(ctx, minijailPath, args...)
		cmdStr := shutil.EscapeSlice(cmd.Args)
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
