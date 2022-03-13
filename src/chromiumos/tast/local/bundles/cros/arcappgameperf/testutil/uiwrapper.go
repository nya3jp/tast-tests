// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package testutil provides utility functions for writing game performance tests.
// TODO(b/224364446): Move shared logic into common folder.
package testutil

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
)

// Click returns an action function which clicks the UI object if it exists.
func Click(obj *ui.Object) action.Action {
	return func(ctx context.Context) error {
		if err := obj.Click(ctx); err != nil {
			return errors.Wrap(err, "failed to click the target object")
		}

		return nil
	}
}

// WaitForExists returns an action function which waits for Android ui object.
func WaitForExists(obj *ui.Object, timeout time.Duration) action.Action {
	return func(ctx context.Context) error {
		return obj.WaitForExists(ctx, timeout)
	}
}
