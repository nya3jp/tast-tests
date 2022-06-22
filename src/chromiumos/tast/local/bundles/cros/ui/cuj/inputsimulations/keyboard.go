// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputsimulations

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/input"
)

// RepeatKeyPressFor presses the specified key repeatedly, with a |delay| between key presses.
func RepeatKeyPressFor(ctx context.Context, kw *input.KeyboardEventWriter, key string, delay, duration time.Duration) error {
	return uiauto.New(nil).WithInterval(delay).WithTimeout(duration+time.Minute).RetryUntil(
		kw.AccelAction(key),
		targetTime(time.Now().Add(duration)),
	)(ctx)
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

func targetTime(t time.Time) action.Action {
	return func(ctx context.Context) error {
		if time.Now().Before(t) {
			return errors.New("target time not reached yet")
		}
		return nil
	}
}
