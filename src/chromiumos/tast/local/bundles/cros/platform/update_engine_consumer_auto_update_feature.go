// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/updateengine"
	"chromiumos/tast/testing"
)

type featuresTestParam struct {
	enabled bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func: UpdateEngineConsumerAutoUpdateFeature,
		Desc: "Verifies that consumer auto update feature work as intended",
		Contacts: []string{
			"kimjae@chromium.org",
			"chromeos-core-services@google.com",
		},
		Fixture: "updateEngineReady",
		Attr:    []string{"group:mainline"},
		Timeout: 3 * time.Minute,
		Params: []testing.Param{
			{
				Name: "enabled",
				Val: featuresTestParam{
					enabled: true,
				},
				ExtraAttr: []string{"informational"},
			},
			{
				Name: "disabled",
				Val: featuresTestParam{
					enabled: false,
				},
				ExtraAttr: []string{"informational"},
			},
		},
	})
}

func UpdateEngineConsumerAutoUpdateFeature(ctx context.Context, s *testing.State) {
	enabled := s.Param().(featuresTestParam).enabled
	if err := setupConsumerAutoUpdate(ctx, updateengine.ConsumerAutoUpdate, enabled); err != nil {
		s.Fatal("Failed to setup test: ", err)
	}
	if enabled {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if status, err := updateengine.Status(ctx); err != nil {
				return testing.PollBreak(err)
			} else if status.LastCheckedTime == 0 {
				return errors.New("Update check was not performed when consumer auto update was turned on")
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second * 10}); err != nil {
			s.Fatal("Failed enabled test: ", err)
		}
	} else {
		if status, err := updateengine.Status(ctx); err != nil {
			s.Fatal("Failed disabled test: ", err)
		} else if status.LastCheckedTime != 0 {
			s.Fatal("Failed disabled test: ", errors.New("Update check performed when consumer auto update was turned off"))
		}
	}
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
	if v, err := updateengine.FeatureEnabled(ctx, feature); err != nil {
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
