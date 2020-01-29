// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// DisableService stops a service if it is running.
func DisableService(ctx context.Context, name string) Result {
	if !upstart.JobExists(ctx, name) {
		testing.ContextLogf(ctx, "Not stopping service %q, doesn't exist", name)
		return ResultNoCleanup()
	}
	goal, _, _, err := upstart.JobStatus(ctx, name)
	if err != nil {
		return ResultFailed(errors.Wrapf(err, "unable to get service status %q", name))
	}
	if goal == upstart.StopGoal {
		testing.ContextLogf(ctx, "Not stopping service %q, not running", name)
		return ResultNoCleanup()
	}
	if err := upstart.StopJob(ctx, name); err != nil {
		return ResultFailed(errors.Wrapf(err, "unable to stop service %q", name))
	}
	testing.ContextLogf(ctx, "Stopped service %q", name)

	return ResultSucceeded(func(ctx context.Context) error {
		if err := upstart.StartJob(ctx, name); err != nil {
			return errors.Wrapf(err, "unable to restart service %q", name)
		}
		testing.ContextLogf(ctx, "Restarted service %q", name)
		return nil
	})
}
