// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// OpenAppAndGetStartTime launches a new activity, starts it and records start time.
// The caller must close the returned activity when the test is done.
func OpenAppAndGetStartTime(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC,
	pkgName, appName, startActivity string) (time.Duration, *arc.Activity, error) {
	act, err := arc.NewActivity(a, pkgName, startActivity)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "failed to create new activity for %s", startActivity)
	}
	startTime := time.Now()
	// Start() will invoke "am start" and waits for the app to be visible on the Chrome side.
	if err := act.Start(ctx, tconn); err != nil {
		return 0, nil, errors.Wrapf(err, "failed to start %s", appName)
	}

	return time.Since(startTime), act, nil
}

// WaitForExists returns an action function which waits for a view matching the selector to appear.
func WaitForExists(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitForExists(ctx, timeout); err != nil {
			return errors.Wrap(err, "failed to wait for the target object")
		}
		return nil
	}
}

// WaitUntilGone returns an action function which waits for a view matching the selector to disappear.
func WaitUntilGone(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitUntilGone(ctx, timeout); err != nil {
			return errors.Wrap(err, "failed to wait for the target object disappear")
		}
		return nil
	}
}

// FindAndClick returns an action function which finds and clicks Android ui object.
func FindAndClick(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		if err := obj.WaitForExists(ctx, timeout); err != nil {
			return errors.Wrap(err, "failed to find the target object")
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

// DoActionUntilExists returns an action function that will repeat the action until the UI object exists.
func DoActionUntilExists(act action.Action, obj *ui.Object, options *testing.PollOptions) action.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if err := obj.Exists(ctx); err != nil {
				act(ctx)
				return err
			}
			return nil
		}, options)
	}
}
