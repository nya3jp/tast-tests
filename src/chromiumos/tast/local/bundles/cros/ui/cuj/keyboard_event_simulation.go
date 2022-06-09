// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/input"
)

// RepeatKeyPressFor presses the specified key repeatedly, with a delay between key presses.
func RepeatKeyPressFor(ctx context.Context, kw *input.KeyboardEventWriter, key string, delay, duration time.Duration) error {
	return runActionFor(ctx, duration, action.Combine(
		"press ["+key+"] and sleep",
		kw.AccelAction(key),
		action.Sleep(delay)))
}

func runActionFor(ctx context.Context, minDuration time.Duration, a action.Action) error {
	for endTime := time.Now().Add(minDuration); time.Now().Before(endTime); {
		if err := a(ctx); err != nil {
			return err
		}
	}
	return nil
}
