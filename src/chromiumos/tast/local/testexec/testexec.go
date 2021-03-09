// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testexec is a wrapper of the standard os/exec package optimized for
// use cases of Tast. Tast tests should always use this package instead of
// os/exec.
//
// This package is designed to be a drop-in replacement of os/exec. Just
// rewriting imports should work. In addition, several methods are available,
// such as Kill and DumpLog.
//
// Features
//
// Automatic log collection. os/exec sends stdout/stderr to /dev/null unless
// explicitly specified to collect them. This default behavior makes it very
// difficult to debug external command failures. This wrapper automatically
// collects those uncaptured logs and allows to log them later.
//
// Process group handling. On timeout, os/exec kills the direct child process
// only. This can often leave orphaned subprocesses in DUT and interfere with
// later tests. To avoid this issue, this wrapper will kill the whole tree
// of subprocesses on timeout by setting process group ID appropriately.
//
// Usage
//
//  cmd := testexec.CommandContext(ctx, "some", "external", "command")
//  if err := cmd.Run(testexec.DumpLogOnError); err != nil {
//      return err
//  }
// Deprecated: use common/testexec instead.
package testexec

import (
	"context"
	"syscall"

	"chromiumos/tast/common/testexec"
)

// Cmd represents an external command being prepared or run.
//
// This struct embeds Cmd in os/exec.
//
// Callers may wish to use shutil.EscapeSlice when logging Args.
// Deprecated: use common/testexec instead.
type Cmd = testexec.Cmd

// RunOption is enum of options which can be passed to Run, Output,
// CombinedOutput and Wait to control precise behavior of them.
// Deprecated: use common/testexec instead.
type RunOption = testexec.RunOption

// DumpLogOnError is an option to dump logs if the executed command fails
// (i.e., exited with non-zero status code).
// Deprecated: use common/testexec instead.
const DumpLogOnError = testexec.DumpLogOnError

// CommandContext prepares to run an external command.
//
// Timeout set in ctx is honored on running the command.
//
// See os/exec package for details.
// Deprecated: use common/testexec instead.
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	return testexec.CommandContext(ctx, name, arg...)
}

// GetWaitStatus extracts WaitStatus from error.
// WaitStatus is typically returned from Run, Output, CombinedOutput and Wait to
// indicate a child process's exit status.
// If err is nil, it returns WaitStatus representing successful exit.
// Deprecated: use common/testexec instead.
func GetWaitStatus(err error) (status syscall.WaitStatus, ok bool) {
	return testexec.GetWaitStatus(err)
}

// ExitCode extracts exit code from error returned by exec.Command.Run().
// Returns exit code and true when succcess. (0, false) otherwise.
// Deprecated: use common/testexec instead.
func ExitCode(cmdErr error) (int, bool) {
	return testexec.ExitCode(cmdErr)
}
