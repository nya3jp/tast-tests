// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
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

// FindAndClick finds and clicks Android ui object.
func FindAndClick(ctx context.Context, obj *ui.Object, timeout time.Duration) error {
	if err := obj.WaitForExists(ctx, timeout); err != nil {
		return errors.Wrap(err, "failed to find the target object")
	}
	if err := obj.Click(ctx); err != nil {
		return errors.Wrap(err, "failed to click the target object")
	}
	return nil
}

// ClickIfExist clicks the prompts if exist.
func ClickIfExist(ctx context.Context, obj *ui.Object, timeout time.Duration) error {
	if err := obj.WaitForExists(ctx, timeout); err == nil {
		if err := obj.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the target object")
		}
	}
	// If there's a prompts pop up but can not find it, ignore if it's not affect the case.
	return nil
}
