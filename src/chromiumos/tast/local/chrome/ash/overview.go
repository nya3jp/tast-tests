// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// WaitForOverviewModeState queries the current overview mode state and wait
// until it fully enters into the target state.
func WaitForOverviewModeState(ctx context.Context, tconn *chrome.Conn, inOverview bool) error {
	expected := "NotInOverview"
	if inOverview {
		expected = "InOverview"
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		got := ""
		if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.getOverviewModeState)()`, &got); err != nil {
			return testing.PollBreak(err)
		}
		if got != expected {
			return errors.Errorf("expected %s, got %s", expected, got)
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond}); err != nil {
		return err
	}
	return nil
}
