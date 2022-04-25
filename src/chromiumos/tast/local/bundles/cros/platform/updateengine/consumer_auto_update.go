// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides functionality to test update_engine/auto update related cases.
package updateengine

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/updateengine"
	"chromiumos/tast/testing"
)

// ValidateConsumerAutoUpdate helps verify consumer auto update feature in update_engine.
func ValidateConsumerAutoUpdate(ctx context.Context, feature updateengine.Feature, enabled bool) error {
	testing.ContextLog(ctx, "Verifying Consumer Auto Update enabled=", enabled)
	if err := setupConsumerAutoUpdate(ctx, feature, enabled); err != nil {
		return err
	}
	if enabled {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if status, err := updateengine.Status(ctx); err != nil {
				return testing.PollBreak(err)
			} else if status.LastCheckedTime == 0 {
				return errors.New("Update check was not performed when consumer auto update was turned off")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second * 10}); err != nil {
			return err
		}
	} else {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if status, err := updateengine.Status(ctx); err != nil {
				return testing.PollBreak(err)
			} else if status.LastCheckedTime != 0 {
				return testing.PollBreak(errors.New("Update check performed when consumer auto update was turned off"))
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second * 10}); err != nil {
			return err
		}
	}

	return nil
}

func setupConsumerAutoUpdate(ctx context.Context, feature updateengine.Feature, enabled bool) error {
	// Mark being OOBE complete, so update-engine won't block update checks.
	if err := updateengine.MarkOobeCompletion(ctx); err != nil {
		return err
	}

	// Toggle consumer auto updates.
	if err := updateengine.ToggleFeature(ctx, feature, enabled); err != nil {
		return err
	}

	// Verify that consumer auto update.
	if v, err := updateengine.IsFeatureEnabled(ctx, feature); err != nil {
		return err
	} else if v != enabled {
		return errors.New("failed to toggle consumer auto update feature")
	}

	// Temporarily stop update-engine to override background update check interval.
	if err := updateengine.StopDaemon(ctx); err != nil {
		return err
	}

	// No delay in performing background update check.
	if err := updateengine.SetPref(ctx, updateengine.TestUpdateCheckIntervalTimeout, "0"); err != nil {
		return err
	}
	if err := updateengine.StartDaemon(ctx); err != nil {
		return err
	}
	if err := updateengine.WaitForService(ctx); err != nil {
		return err
	}

	return nil
}
