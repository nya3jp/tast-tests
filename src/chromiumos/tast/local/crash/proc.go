// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

// WaitForProcessEnd waits until all processes that match pattern by process name ends.
func WaitForProcessEnd(ctx context.Context, name string) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		cmd := testexec.CommandContext(ctx, "pgrep", name)
		err := cmd.Run()
		if cmd.ProcessState == nil {
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Wrapf(err, "failed to get exit code of %s", name))
		}
		if code := (cmd.ProcessState).ExitCode(); code == 0 {
			// pgrep return code 0: one or more process matched
			return errors.Errorf("still have a %s process", name)
		} else if code != 1 {
			cmd.DumpLog(ctx)
			return testing.PollBreak(errors.Errorf("unexpected return code: %d", code))
		}
		// pgrep return code 1: no process matched
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
