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
		Contacts:     []string{"iby@chromium.org", "chromeos-ui@google.com", "cros-monitoring-forensics@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "breakpad",
			Val:               chromecrash.Breakpad,
			ExtraSoftwareDeps: []string{"breakpad"},
		}, {
			Name:      "crashpad",
			Val:       chromecrash.Crashpad,
			ExtraAttr: []string{"informational"},
		}},
	})
}

func ChromeCrashLoggedInDirect(ctx context.Context, s *testing.State) {
	// This is the old test, left here so that we don't lose test coverage while
	// waiting for ChromeCrashLoggedIn to be stable enough to promote to a
	// critical (non-informational) test.
	// TODO(crbug.com/984807): Once ChromeCrashLoggedIn is no longer "informational",
	// remove this test.
	// We use crash.DevImage() here because this test still uses the testing
	// command-line flags on crash_reporter to bypass metrics consent and such.
	// Those command-line flags only work if the crash-test-in-progress does not
	// exist.
	if err := crash.SetUpCrashTest(ctx, crash.DevImage()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer func() {
		if err := crash.TearDownCrashTest(); err != nil {
			s.Error("Failed to tear down crash test: ", err)
		}
	}()

	ct, err := chromecrash.NewCrashTester(chromecrash.Browser, chromecrash.BreakpadDmp)
	if err != nil {
		s.Fatal("NewCrashTester failed: ", err)
	}
	defer ct.Close()

	handler := s.Param().(chromecrash.CrashHandler)
	cr, err := chrome.New(ctx, chrome.ExtraArgs(chromecrash.GetExtraArgs(handler)...))
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
