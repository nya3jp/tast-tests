// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package apputil implements the libraries used to control ARC apps
package apputil

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
)

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

// WaitForExists returns an action function which wait for Android ui object.
func WaitForExists(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		return obj.WaitForExists(ctx, timeout)
	}
}
