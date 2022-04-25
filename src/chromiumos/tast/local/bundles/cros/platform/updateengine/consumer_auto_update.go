// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package updateengine provides functionality to test update_engine/auto update related cases.
package updateengine

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/updateengine"
	"chromiumos/tast/testing"
)

const (
	oobeCompletedFlag     = "/home/chronos/.oobe_completed"
	oobeCompletedFlagPerm = 0644
)

// ValidateConsumerAutoUpdate helps verify consumer auto update feature in update_engine.
func ValidateConsumerAutoUpdate(ctx context.Context, feature updateengine.Feature) error {
	testing.ContextLog(ctx, "Verifying Consumer Auto Update turned on")
	if err := setupConsumerAutoUpdate(ctx, feature, true); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := updateengine.Status(ctx); err != nil {
			return err
		} else if status.LastCheckedTime == 0 {
			return errors.New("Update check was not performed when consumer auto update was turned off")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 10}); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Verifying Consumer Auto Update turned off")
	if err := setupConsumerAutoUpdate(ctx, feature, false); err != nil {
		return err
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := updateengine.Status(ctx); err != nil {
			return err
		} else if status.LastCheckedTime != 0 {
			return testing.PollBreak(errors.New("Update check performed when consumer auto update was turned off"))
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second * 10}); err != nil {
		return err
	}

	return nil
}

func setupConsumerAutoUpdate(ctx context.Context, feature updateengine.Feature, enabled bool) error {
	// Fake being OOBE complete, so update-engine won't block update checks.
	if _, err := os.OpenFile(oobeCompletedFlag, os.O_RDONLY|os.O_CREATE, oobeCompletedFlagPerm); err != nil {
		return errors.Wrap(err, "failed to touch OOBE completed flag")
	}

	// Wipe all previous/old prefs to start test fresh.
	if err := updateengine.ClearPrefs(ctx); err != nil {
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
		return errors.New("Consumer auto update failed to toggle")
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
