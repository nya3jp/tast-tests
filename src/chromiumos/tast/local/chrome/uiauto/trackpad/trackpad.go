// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trackpad provides helper functions to simulate trackpad events.
package trackpad

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/settings"
	"chromiumos/tast/local/input"
)

// SwipeDirection represents the swipe direction.
type SwipeDirection int

// Variables used to determine the swipe direction.
const (
	UpSwipe SwipeDirection = iota
	DownSwipe
	LeftSwipe
	RightSwipe
)

// Swipe performs a swipe movement in the indicated direction with given number of touches.
func Swipe(ctx context.Context, tconn *chrome.TestConn, tpw *input.TrackpadEventWriter, swipeDirection SwipeDirection, touches int) error {
	const (
		swipeDuration            = 250 * time.Millisecond
		fingerSeparationVertical = input.TouchCoord(0)
	)
	fingerSeparationHorizontal := tpw.Width() / 16

	if err := trackpadValid(ctx, tconn); err != nil {
		return errors.Wrap(err, "trackpad cannot cannot be used")
	}

	mtw, err := tpw.NewMultiTouchWriter(touches)
	if err != nil {
		return errors.Wrapf(err, "unable to create touchpad with %d touches", touches)
	}
	defer mtw.Close()

	var x0, y0, x1, y1 input.TouchCoord
	centerX := input.TouchCoord(tpw.Width() / 2)
	centerY := input.TouchCoord(tpw.Height() / 2)

	switch swipeDirection {
	case UpSwipe:
		x0 = centerX
		x1 = x0
		y0 = tpw.Height() - 1
		y1 = 1
	case DownSwipe:
		x0 = centerX
		x1 = x0
		y0 = 1
		y1 = tpw.Height() - 1
	case LeftSwipe:
		y0 = centerY
		y1 = y0
		x0 = centerX
		x1 = 1
	case RightSwipe:
		y0 = centerY
		y1 = y0
		x0 = 1
		x1 = centerX
	default:
		return errors.Errorf("invalid swipe direction: %v", swipeDirection)
	}

	// Perform swiping.
	return mtw.Swipe(ctx, x0, y0, x1, y1, fingerSeparationHorizontal, fingerSeparationVertical, touches, swipeDuration)

}

// TurnOnReverseScroll turns on the reverse scrolling for trackpad.
func TurnOnReverseScroll(ctx context.Context, tconn *chrome.TestConn) error {
	if err := settings.SetTrackpadReverseScrollEnabled(ctx, tconn, true); err != nil {
		return errors.Wrap(err, "failed to set trackpad reverse scroll enabled")
	}

	enabled, err := settings.TrackpadReverseScrollEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get current trackpad reverse scroll state")
	}

	if !enabled {
		return errors.New("the trackpad reverse scroll state is not expected")
	}

	return nil
}

// TurnOffReverseScroll turns off the reverse scrolling for trackpad.
func TurnOffReverseScroll(ctx context.Context, tconn *chrome.TestConn) error {
	if err := settings.SetTrackpadReverseScrollEnabled(ctx, tconn, false); err != nil {
		return errors.Wrap(err, "failed to set trackpad reverse scroll disabled")
	}

	enabled, err := settings.TrackpadReverseScrollEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get current trackpad reverse scroll state")
	}

	if enabled {
		return errors.New("the trackpad reverse scroll state is not expected")
	}

	return nil
}

// trackpadValid checks if trackpad can be used in tests.
func trackpadValid(ctx context.Context, tconn *chrome.TestConn) error {
	// The device cannot be in tablet mode.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "unable to determine tablet mode")
	}

	if tabletModeEnabled {
		return errors.New("Device is in tablet mode, cannot use trackpad")
	}

	return nil
}
