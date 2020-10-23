// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ambient supports interaction with ChromeOS Ambient mode.
package ambient

import (
	"context"
	"time"

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

// SetTimeouts changes timeouts for Ambient mode to speed up testing. Rounds
// values to the nearest second.
func SetTimeouts(
	ctx context.Context,
	tconn *chrome.TestConn,
	timeouts Timeouts,
) error {
	return tconn.Call(
		ctx,
		nil,
		`tast.promisify(chrome.autotestPrivate.setAmbientTimeouts)`,
		toNearestSecond(timeouts.LockScreenIdle),
		toNearestSecond(timeouts.BackgroundLockScreen),
		toNearestSecond(timeouts.PhotoRefreshInterval),
	)
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
