// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lockscreen is used to get information about the login screen directly through the UI.
package lockscreen

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// State contains a subset of the state returned by chrome.autotestPrivate.loginStatus.
type State struct {
	Locked bool `json:"isScreenLocked"`
	Ready  bool `json:"isReadyForPassword"`
}

// WaitState repeatedly calls chrome.autotestPrivate.loginStatus and passes the returned
// state to check until it returns true or timeout is reached. The last-seen state is returned.
func WaitState(ctx context.Context, tconn *chrome.TestConn, check func(st State) bool, timeout time.Duration) (State, error) {
	var st State
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.EvalPromise(ctx, `tast.promisify(chrome.autotestPrivate.loginStatus)()`, &st); err != nil {
			return err
		} else if !check(st) {
			return errors.New("wrong status")
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
	return st, err
}
