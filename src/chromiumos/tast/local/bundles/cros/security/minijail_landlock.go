// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MinijailLandlock,
		Desc: "Verifies minijail0's Landlock enforcement",
		Contacts: []string{
			"akhna@google.com",
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"landlock_enabled"},
		Attr:         []string{"group:mainline"},
	})
}

func MinijailLandlock(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"
		exitSuccess  = 0
	)
	landlockArgs := []string{"--fs-default-paths", "--fs-path-rx=/usr/local/libexec/tast"}

	for _, tc := range []struct {
		name string   // human-readable test case name
		cmd  string   // shell-quoted command and arguments to run via "bash -c"
		args []string // minijail0-specific args
	}{
		{
			"landlock-allow-nonzero-return",
			"/bin/false",
			landlockArgs,
		},
		{
			"landlock-deny-disallowed-path",
			"/bin/ls /dev",
			landlockArgs,
		},
	} {
		if ctx.Err() != nil {
			s.Error("Aborting testing: ", ctx.Err())
			break
		}
		var args []string
		args = append(args, tc.cmd)
		args = append(args, tc.args...)
		cmd := testexec.CommandContext(ctx, minijailPath, args...)
		cmdStr := shutil.EscapeSlice(cmd.Args)
		s.Logf("Running %q: %v", tc.name, cmdStr)
		err := cmd.Run()

		if st, ok := testexec.GetWaitStatus(err); !ok {
			s.Errorf("Case %q (%v) failed (no exit status): %v", tc.name, cmdStr, err)
			cmd.DumpLog(ctx)
		} else if st.ExitStatus() == exitSuccess {
			s.Errorf("Case %q (%v) exited with %d; want nonzero", tc.name, cmdStr, st.ExitStatus())
			cmd.DumpLog(ctx)
		}
	}
}
