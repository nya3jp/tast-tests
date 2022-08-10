// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		swipeDistanceAsProportionOfPad = .4
		swipeDuration                  = 250 * time.Millisecond
		fingerSeparationHorizontal     = input.TouchCoord(5)
		fingerSeparationVertical       = input.TouchCoord(0)
	)

	if err := trackpadValid(ctx, tconn); err != nil {
		return errors.Wrap(err, "trackpad cannot cannot be used")
	}

	mtw, err := tpw.NewMultiTouchWriter(touches)
	if err != nil {
		return errors.Wrapf(err, "unable to create touchpad with %d touches", touches)
	}
	defer mtw.Close()

	// Start swipe from the center of the trackpad.
	x0 := input.TouchCoord(tpw.Width() / 2)
	y0 := input.TouchCoord(tpw.Height() / 2)

	// Estmate the horizontal and vertical swipe distance.
	horizontalSwipeDistance := input.TouchCoord(float64(tpw.Width()) * swipeDistanceAsProportionOfPad)
	verticalSwipeDistance := input.TouchCoord(float64(tpw.Height()) * swipeDistanceAsProportionOfPad)

	// Calculate the end point.
	x1 := x0
	y1 := y0
	switch swipeDirection {
	case UpSwipe:
		y1 = y0 - verticalSwipeDistance
	case DownSwipe:
		y1 = y0 + verticalSwipeDistance
	case LeftSwipe:
		x1 = x0 - horizontalSwipeDistance
	case RightSwipe:
		x1 = x0 + horizontalSwipeDistance
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
