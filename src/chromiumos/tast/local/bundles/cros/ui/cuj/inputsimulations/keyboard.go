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

// RepeatKeyPressFor presses the specified key repeatedly, with a delay between key presses.
func RepeatKeyPressFor(ctx context.Context, kw *input.KeyboardEventWriter, key string, delay, duration time.Duration) error {
	return uiauto.New(nil).WithInterval(delay).WithTimeout(duration+time.Minute).RetryUntil(
		kw.AccelAction(key),
		targetTime(time.Now().Add(duration)),
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
