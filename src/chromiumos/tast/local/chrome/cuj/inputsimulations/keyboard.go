// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputsimulations

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// RepeatKeyPressFor presses the specified key repeatedly, with a |delay| between key presses.
func RepeatKeyPressFor(ctx context.Context, kw *input.KeyboardEventWriter, key string, delay, duration time.Duration) error {
	return runActionFor(ctx, duration, action.Combine(
		"press ["+key+"] and sleep",
		kw.AccelAction(key),
		action.Sleep(delay)))
}

// RepeatKeyPress presses the specified key |n| times, with a |delay| between key presses.
func RepeatKeyPress(ctx context.Context, kw *input.KeyboardEventWriter, key string, delay time.Duration, n int) error {
	return uiauto.Repeat(
		n,
		action.Combine("press "+key+" and sleep",
			kw.AccelAction(key),
			action.Sleep(delay),
		),
	)(ctx)
}

// RepeatKeyHold holds a key down for |keyHoldDuration| |n| times,
// with a |delay| between each key release and the next key press.
func RepeatKeyHold(ctx context.Context, kw *input.KeyboardEventWriter, key string, keyHoldDuration, delay time.Duration, n int) error {
	return uiauto.Repeat(
		n,
		action.Combine("press and hold "+key+" and sleep",
			kw.AccelPressAction(key),
			action.Sleep(keyHoldDuration),
			kw.AccelReleaseAction(key),
			action.Sleep(delay),
		),
	)(ctx)
}

func runActionFor(ctx context.Context, minDuration time.Duration, a action.Action) error {
	for endTime := time.Now().Add(minDuration); time.Now().Before(endTime); {
		if err := a(ctx); err != nil {
			return err
		}
	}
	return nil
}
