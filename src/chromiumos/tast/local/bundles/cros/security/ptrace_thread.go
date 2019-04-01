// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"strconv"
	"syscall"

	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PtraceThread,
		Desc: "Checks that the kernel restricts ptrace between threads",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
	})
}

func PtraceThread(ctx context.Context, s *testing.State) {
	// TODO(derat): Consider moving the base helper path to a shared constant somewhere.
	const threadPrctlPath = "/usr/local/libexec/tast/helpers/local/cros/security.PtraceThread.thread-prctl"

	// See the thread-prctl executable installed by the security_tests package for details.
	type behavior int
	const (
		tracerForksTracee                   behavior = 0
		traceeCallsPrctlFromMainProcess              = 1
		traceeCallsPrctlFromNonLeaderThread          = 2
	)

	type source int
	const (
		ptraceFromNonLeaderThread source = 0
		ptraceFromMainProcess            = 1
	)

	runThreadTest := func(desc string, bh behavior, src source) {
		// Return early if we've already timed out.
		if ctx.Err() != nil {
			return
		}

		s.Log("Testing ", desc)
		cmd := testexec.CommandContext(ctx, threadPrctlPath, strconv.Itoa(int(bh)), strconv.Itoa(int(src)))
		cmd.Cred(syscall.Credential{Uid: sysutil.ChronosUID, Gid: sysutil.ChronosGID})
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("%v failed: %v", desc, err)
		}
	}

	runThreadTest("ptrace child from parent", tracerForksTracee, ptraceFromMainProcess)
	runThreadTest("ptrace child from thread", tracerForksTracee, ptraceFromNonLeaderThread)

	runThreadTest("PR_SET_PTRACER from main process", traceeCallsPrctlFromMainProcess, ptraceFromMainProcess)
	runThreadTest("PR_SET_PTRACER from thread", traceeCallsPrctlFromNonLeaderThread, ptraceFromMainProcess)

	runThreadTest("ptrace from main process after PR_SET_PTRACER", traceeCallsPrctlFromMainProcess, ptraceFromNonLeaderThread)
	runThreadTest("ptrace from thread after PR_SET_PTRACER", traceeCallsPrctlFromNonLeaderThread, ptraceFromNonLeaderThread)
}
