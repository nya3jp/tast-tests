// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// RepeatKeyPressFor presses the specified key repeatedly with a delay between each key press.
// This action is repeated until the amount of time specified by the duration parameter has passed.
func RepeatKeyPressFor(ctx context.Context, kw *input.KeyboardEventWriter, key string, delay, duration time.Duration) error {
	return runActionFor(ctx, duration, uiauto.Combine(
		"press ["+key+"] and sleep",
		kw.AccelAction(key),
		action.Sleep(delay)))
}

func runActionFor(ctx context.Context, minDuration time.Duration, action func(ctx context.Context) error) error {
	for endTime := time.Now().Add(minDuration); time.Now().Before(endTime); {
		if err := action(ctx); err != nil {
			return err
		}
	}
	return nil
}
