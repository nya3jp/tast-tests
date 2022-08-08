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

// SetOverviewModeAndWait requests Ash to set the overview mode state and waits
// for its animation to complete.
func SetOverviewModeAndWait(ctx context.Context, tconn *chrome.TestConn, inOverview bool) error {
	finished := false
	if err := tconn.Call(ctx, &finished, "tast.promisify(chrome.autotestPrivate.setOverviewModeState)", inOverview); err != nil {
		return err
	}
	if !finished {
		return errors.New("the overview mode animation is canceled")
	}
	return nil
}

// SetOverviewModeAndWaitWithTimeout sets the overview mode state and waits
// until a given timeout.
func SetOverviewModeAndWaitWithTimeout(ctx context.Context, tconn *chrome.TestConn, inOverview bool, timeout time.Duration) error {
	finished := false
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Call(ctx, &finished, "tast.promisify(chrome.autotestPrivate.setOverviewModeState)", inOverview); err != nil {
			return testing.PollBreak(err)
		}
		if !finished {
			return errors.New("the overview mode is not yet set")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout}); err != nil {
		return errors.Wrap(err, "Timed out to wait for the overview mode to be set")
	}
	return nil
}
