// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package printer provides utilities about printer/cups.
package printer

import (
	"context"
	"time"

	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// ResetCups removes the privileged directories for cupsd.
// If cupsd is running, this stops it.
func ResetCups(ctx context.Context) error {
	testing.ContextLog(ctx, "Resetting cups")
	// If cups-clear-state is running, the job could fail. Retry then to
	// ensure the directories are properly removed.
	return testing.Poll(ctx, func(ctx context.Context) error {
		// cups-clear-state is a task, so when returned successfully,
		// it is ensured that the directories are cleared.
		return upstart.StartJob(ctx, "cups-clear-state")
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: time.Second})
}
