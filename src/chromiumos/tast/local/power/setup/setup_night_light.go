// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/settings"
	"chromiumos/tast/testing"
)

// SetNightLightEnabled enables or disables Night Light.
func SetNightLightEnabled(ctx context.Context, c *chrome.Conn, enabled bool) (CleanupCallback, error) {
	prevEnabled, err := settings.NightLightEnabled(ctx, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set Night Light enabled")
	}

	testing.ContextLogf(ctx, "Setting Night Light enabled to %t from %t", enabled, prevEnabled)
	if err := settings.SetNightLightEnabled(ctx, c, enabled); err != nil {
		return nil, errors.Wrapf(err, "failed to set Night Light enabled to %t", enabled)
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting Night Light enabled to %t", prevEnabled)
		if err := settings.SetNightLightEnabled(ctx, c, prevEnabled); err != nil {
			return errors.Wrapf(err, "failed to reset Night Light enabled to %t", prevEnabled)
		}
		return nil
	}, nil
}

// SetNightLightSchedule sets the Night Light schedule. See
// settings.NightLightSchedule* for possible values.
func SetNightLightSchedule(ctx context.Context, c *chrome.Conn, schedule uint) (CleanupCallback, error) {
	prevSchedule, err := settings.NightLightSchedule(ctx, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Night Light schedule")
	}

	testing.ContextLogf(ctx, "Setting Night Light schedule to %d from %d", schedule, prevSchedule)
	if err := settings.SetNightLightSchedule(ctx, c, schedule); err != nil {
		return nil, errors.Wrapf(err, "failed to set Night Light schedule to %d", schedule)
	}

	return func(ctx context.Context) error {
		testing.ContextLogf(ctx, "Resetting Night Light schedule to %d", prevSchedule)
		if err := settings.SetNightLightSchedule(ctx, c, prevSchedule); err != nil {
			return errors.Wrapf(err, "failed to reset Night Light schedule to %d", prevSchedule)
		}
		return nil
	}, nil
}

// TurnOffNightLight disables Night Light, and sets the schedule to 'Never' so
// it won't turn on half way through a test.
func TurnOffNightLight(ctx context.Context, c *chrome.Conn) (CleanupCallback, error) {
	return Nested(ctx, "night light", func(s *Setup) error {
		s.Add(SetNightLightSchedule(ctx, c, settings.NightLightScheduleNever))
		s.Add(SetNightLightEnabled(ctx, c, false))
		return nil
	})
}
