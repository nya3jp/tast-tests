// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Functions for testing crash_sender. It includes mocking the crash sender, as well as
// verifying the output of the crash sender.

// ExitCode extracts exit code from error returned by exec.Command.Run().
// Returns exit code and true when succcess. (0, false) otherwise.
// TODO(yamaguchi): Replace this with (*cmd.ProcessState).ExitCode() after golang is uprevved to >= 1.12.
func ExitCode(cmdErr error) (int, bool) {
	s, ok := testexec.GetWaitStatus(cmdErr)
	if !ok {
		return 0, false
	}
	if s.Exited() {
		return s.ExitStatus(), true
	}
	if s.Signaled() {
		return int(s.Signal()) + 128, true
	}
	return 0, false
}

// WaitForProcessEnd waits until a process containing specified name in the full commandline is finished or aborted.
func WaitForProcessEnd(ctx context.Context, name string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", "-f", name)
		err := cmd.Run()
		if err == nil {
			return errors.Errorf("still have a %s process", name)
		}
		if code, ok := ExitCode(err); !ok {
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrapf(err, "failed to get exit code of %s", name))
		} else if code == 0 {
			// This will never happen. If return code is 0, cmd.Run indicates it by err==nil.
			return testing.PollBreak(errors.New("inconsistent results returned from cmd.Run()"))
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Duration(10) * time.Second})
}
