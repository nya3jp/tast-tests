// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScreenLock,
		Desc:         "Checks that screen-locking works in Chrome",
		Contacts:     []string{"derat@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func ScreenLock(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	const lockExpr = "chrome.autotestPrivate.lockScreen()"
	s.Log("Locking screen via ", lockExpr)
	if err := conn.Exec(ctx, lockExpr); err != nil {
		s.Fatalf("Calling %v failed: %v", lockExpr, err)
	}

	s.Log("Waiting for Chrome to report that screen is locked")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const checkExpr = `
			new Promise((resolve) => {
			  chrome.autotestPrivate.loginStatus((status) => {
			    resolve(status.isScreenLocked);
			  });
			})`
		locked := false
		if err := conn.EvalPromise(ctx, checkExpr, &locked); err != nil {
			return err
		} else if !locked {
			return errors.New("screen not locked")
		}
		s.Log("Screen is locked")
		return nil
	}, nil); err != nil {
		s.Fatal("Waiting for screen to be locked failed: ", err)
	}
}
