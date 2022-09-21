// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apputil implements the libraries used to control ARC apps
package apputil

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

// FindAndClick returns an action function which finds and clicks Android ui object.
func FindAndClick(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitForExists(ctx, timeout); err != nil {
			return errors.Wrap(err, "failed to wait for target object exists")
		}
		if err := obj.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the target object")
		}
		return nil
	}
}

// ClickIfExist returns an action function which clicks the UI object if it exists.
func ClickIfExist(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitForExists(ctx, timeout); err != nil {
			if ui.IsTimeout(err) {
				return nil
			}
			return errors.Wrap(err, "failed to wait for the target object")
		}
		return obj.Click(ctx)
	}
}

// WaitForExists returns an action function which wait for Android ui object.
func WaitForExists(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		return obj.WaitForExists(ctx, timeout)
	}
}

// CheckObjectExists waits and checks the UI object's existence.
// It returns true if found otherwise false.
func CheckObjectExists(ctx context.Context, obj *ui.Object, timeout time.Duration) (bool, error) {
	if err := obj.WaitForExists(ctx, timeout); err != nil {
		if ui.IsTimeout(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "failed to check if object exists")
	}
	return true, nil
}

// ClickAnyFromObjectPool clicks the first (randomly) found object from the given object pool,
// returns an error if none of the objects were found and clicked.
// pool specifies the map of the object and its description (for debug purpose).
// timeout specifies the maximum time duration to wait and check on each object.
//
// This function is an utility designed for following purposes:
//	1. Handling UI operation on an ARC app that is performing A/B testing.
//	2. Handling multiple different objects with the same purposes or outcome.
func ClickAnyFromObjectPool(ctx context.Context, pool map[*ui.Object]string, timeout time.Duration) error {
	clicked := false

	for btn, description := range pool {
		if exist, err := CheckObjectExists(ctx, btn, timeout); err != nil {
			return errors.Wrap(err, "failed to check if object exist")
		} else if !exist {
			continue
		}

		if err := btn.Click(ctx); err != nil {
			return errors.Wrapf(err, "failed to click %s", description)
		}
		testing.ContextLogf(ctx, "Object %q clicked", description)
		clicked = true
		break
	}

	if !clicked {
		return errors.New("failed to click any button from given object pool")
	}

	return nil
}

// WaitUntilGone waits for a view matching the selector to disappear.
func WaitUntilGone(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		return obj.WaitUntilGone(ctx, timeout)
	}
}

// SwipeRight performs the swipe right action on the UiObject.
func SwipeRight(obj *ui.Object, steps int) action.Action {
	return func(ctx context.Context) error {
		return obj.SwipeRight(ctx, steps)
	}
}

// DragAndDrop performs drag at start point to end point via ADB command.
func DragAndDrop(a *arc.ARC, start, end coords.Point, duration time.Duration) action.Action {
	return func(ctx context.Context) error {
		speed := int(duration.Milliseconds())
		return a.Command(ctx, "input", "draganddrop", strconv.Itoa(start.X), strconv.Itoa(start.Y), strconv.Itoa(end.X), strconv.Itoa(end.Y), strconv.Itoa(speed)).Run(testexec.DumpLogOnError)
	}
}
