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
func DisableService(ctx context.Context, name string) (CleanupCallback, error) {
	if !upstart.JobExists(ctx, name) {
		return nil, errors.Errorf("service %q does not exist", name)
	}
	goal, _, _, err := upstart.JobStatus(ctx, name)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get service status %q", name)
	}
	if goal == upstart.StopGoal {
		testing.ContextLogf(ctx, "Not stopping service %q, not running", name)
		return nil, nil
	}
	testing.ContextLogf(ctx, "Stopping service %q", name)
	if err := upstart.StopJob(ctx, name); err != nil {
		return nil, errors.Wrapf(err, "unable to stop service %q", name)
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Restarting service %q", name)
		return upstart.StartJob(ctx, name)
	}, nil
}

// DisableServiceIfExists stops a service if it is running. Unlike
// DisableService above, it does not return an error if the service does not
// exist.
func DisableServiceIfExists(ctx context.Context, name string) (CleanupCallback, error) {
	if !upstart.JobExists(ctx, name) {
		return nil, nil
	}
	return DisableService(ctx, name)
}
