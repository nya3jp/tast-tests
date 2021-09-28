// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package standardizedtestutil

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/input"
)

// MouseInputDevice abstracts away mouse related implementation details
// and provides an interface for sending high level mouse actions to Android
// activities in a standardized, and reliable way.
type MouseInputDevice struct {
	ctx            context.Context
	testParameters TestFuncParams
	mew            *input.MouseEventWriter
}

// NewMouseInputDevice creates a new MouseInputDevice instance.
func NewMouseInputDevice(ctx context.Context, testParameters TestFuncParams) (*MouseInputDevice, error) {
	if err := validatePointerCanBeUsed(ctx, testParameters); err != nil {
		return nil, errors.Wrap(err, "the mouse cannot be used")
	}

	// Setup the mouse.
	mew, err := input.Mouse(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to setup the mouse")
	}

	return &MouseInputDevice{
		ctx:            ctx,
		testParameters: testParameters,
		mew:            mew,
	}, nil
}

// Close closes the mouse input device and frees all resources.
func (mid *MouseInputDevice) Close() error {
	if mid.mew != nil {
		return mid.mew.Close()
	}

	return nil
}

// ClickObject clicks the provided mouse button an element.
func (mid *MouseInputDevice) ClickObject(selector *ui.Object, mouseButton PointerButton) error {
	// Move the mouse into position.
	if err := centerPointerOnObject(mid.ctx, mid.testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the mouse into position")
	}

	// Perform the correct click
	switch mouseButton {
	case LeftPointerButton:
		if err := mid.mew.Click(); err != nil {
			return errors.Wrap(err, "unable to perform left mouse click")
		}
	case RightPointerButton:
		if err := mid.mew.RightClick(); err != nil {
			return errors.Wrap(err, "unable to perform right mouse click")
		}
	default:
		return errors.Errorf("invalid button; got: %v", mouseButton)
	}

	return nil
}

// MoveOntoObject moves the mouse onto the center of an object.
func (mid *MouseInputDevice) MoveOntoObject(selector *ui.Object) error {
	if err := centerPointerOnObject(mid.ctx, mid.testParameters, selector); err != nil {
		return errors.Wrap(err, "failed to move the mouse into position")
	}

	return nil
}

// Scroll performs a scroll on the mouse. Due to different device
// settings, the actual scroll amount in pixels will be imprecise. Therefore,
// multiple iterations should be run, with a check for the desired output
// between each call.
func (mid *MouseInputDevice) Scroll(scrollDirection ScrollDirection) error {
	switch scrollDirection {
	case UpScroll:
		if err := mid.mew.ScrollUp(); err != nil {
			return errors.Wrap(err, "unable to scroll up")
		}
	case DownScroll:
		if err := mid.mew.ScrollDown(); err != nil {
			return errors.Wrap(err, "unable to scroll down")
		}
	default:
		return errors.Errorf("invalid scroll direction; got: %v", scrollDirection)
	}

	return nil
}
