// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashNotLoggedInDirect,
		Desc:         "Checks that Chrome writes crash dumps while not logged in; old version that does not invoke crash_reporter",
		Contacts:     []string{"iby@chromium.org", "chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChromeCrashNotLoggedInDirect(ctx context.Context, s *testing.State) {
	// This is the old test, left here so that we don't lose test coverage while
	// waiting for ChromeCrashNotLoggedIn to be stable enough to promote to a
	// critical (non-informational) test.
	// TODO(crbug.com/984807): Once ChromeCrashNotLoggedIn is no longer "informational",
	// remove this test.
	cr, err := chrome.New(ctx, chrome.NoLogin())
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(ctx)

	if dumps, err := chromecrash.KillAndGetCrashFiles(ctx); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	} else if len(dumps) == 0 {
		s.Error("No minidumps written after not-logged-in Chrome crash")
	}
}
