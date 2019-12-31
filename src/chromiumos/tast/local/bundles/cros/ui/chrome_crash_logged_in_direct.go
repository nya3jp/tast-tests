// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashLoggedInDirect,
		Desc:         "Checks that Chrome writes crash dumps while logged in; old version that does not invoke crash_reporter",
		Contacts:     []string{"iby@chromium.org", "chromeos-ui@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func ChromeCrashLoggedInDirect(ctx context.Context, s *testing.State) {
	// This is the old test, left here so that we don't lose test coverage while
	// waiting for ChromeCrashLoggedIn to be stable enough to promote to a
	// critical (non-informational) test.
	// TODO(crbug.com/984807): Once ChromeCrashLoggedIn is no longer "informational",
	// remove this test.
	if err := crash.SetUpCrashTest(ctx); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	ct, err := chromecrash.NewCrashTester(chromecrash.Browser, chromecrash.BreakpadDmp)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	if dumps, err := ct.KillAndGetCrashFiles(ctx); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	} else if len(dumps) == 0 {
		s.Error("No minidumps written after logged-in Chrome crash")
	}
}
