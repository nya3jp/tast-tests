// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package procutil

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// WaitForTerminated waits for the process p to be terminated.
func WaitForTerminated(ctx context.Context, p *process.Process, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		r, err := p.IsRunning()
		if err == nil && r {
			return errors.Errorf("process %d is still running", p.Pid)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}
