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
			// pgrep return code 0: one or more process matched
			return errors.Errorf("still have a %s process", name)
		}
		if code, ok := ExitCode(err); !ok {
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrapf(err, "failed to get exit code of %s", name))
		} else if code != 1 {
			return testing.PollBreak(errors.Errorf("unexpected return code: %d", code))
		}
		// pgrep return code 1: no process matched
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
