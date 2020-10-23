// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ambient supports interaction with ChromeOS Ambient mode.
package ambient

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// Timeouts contains durations to configure Ambient mode timeouts.
type Timeouts struct {
	LockScreenIdle       time.Duration
	BackgroundLockScreen time.Duration
	PhotoRefreshInterval time.Duration
}

func toNearestSecond(d time.Duration) int {
	return int(d.Round(time.Second).Seconds())
}

// SetEnabled turns the Ambient mode pref on or off.
func SetEnabled(ctx context.Context, tconn *chrome.TestConn, value bool) error {
	return tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"settings.ambient_mode.enabled",
		value,
	)
}

// SetTimeouts changes timeouts for Ambient mode to speed up testing. Rounds
// values to the nearest second.
func SetTimeouts(
	ctx context.Context,
	tconn *chrome.TestConn,
	timeouts Timeouts,
) error {
	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.lock_screen_idle_timeout",
		toNearestSecond(timeouts.LockScreenIdle),
	); err != nil {
		return errors.Wrap(err, "failed to set lock screen idle timeout")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.lock_screen_background_timeout",
		toNearestSecond(timeouts.BackgroundLockScreen),
	); err != nil {
		return errors.Wrap(err, "failed to set lock screen background timeout")
	}

	if err := tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.settingsPrivate.setPref)`,
		"ash.ambient.photo_refresh_interval",
		toNearestSecond(timeouts.PhotoRefreshInterval),
	); err != nil {
		return errors.Wrap(err, "failed to set photo refresh interval")
	}

	return nil
}

// WaitForPhotoTransitions blocks until the desired number of photo transitions
// has occurred.
func WaitForPhotoTransitions(
	ctx context.Context,
	tconn *chrome.TestConn,
	numCompletions int,
	timeout time.Duration,
) error {
	return tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.autotestPrivate.waitForAmbientPhotoAnimation)`,
		numCompletions,
		toNearestSecond(timeout),
	)
}
