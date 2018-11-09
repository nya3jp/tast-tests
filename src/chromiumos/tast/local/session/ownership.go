// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package session

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	// policyPath is a directory containing policy files.
	policyPath = "/var/lib/whitelist"

	// localStatePath is a file containing local state JSON.
	localStatePath = "/home/chronos/Local State"
)

// ClearDeviceOwnership deletes DUT's ownership infomation.
func ClearDeviceOwnership(ctx context.Context) error {
	testing.ContextLog(ctx, "Clearing device owner info")

	// The UI must be stopped while we do this, or the session_manager will
	// write the policy and key files out again.
	if goal, state, _, err := upstart.JobStatus(ctx, "ui"); err != nil {
		return err
	} else if goal != upstart.StopGoal || state != upstart.WaitingState {
		return errors.Errorf("device ownership is being cleared while ui is not stopped: %v/%v", goal, state)
	}

	if err := os.RemoveAll(policyPath); err != nil {
		return errors.Wrapf(err, "failed to remove %s", policyPath)
	}

	if err := os.Remove(localStatePath); err != nil {
		return errors.Wrapf(err, "failed to remove %s", localStatePath)
	}

	return nil
}
