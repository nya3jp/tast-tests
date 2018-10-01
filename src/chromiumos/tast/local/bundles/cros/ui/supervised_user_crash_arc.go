// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/supervised"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SupervisedUserCrashARC,
		Desc:         "Sign in with ARC, indicate that supervised user is being created, then crash",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
	})
}

func SupervisedUserCrashARC(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to login using Chrome: ", err)
	}
	defer cr.Close(ctx)
	supervised.RunTest(ctx, s)
}
