// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeCrashNotLoggedIn,
		Desc:         "Checks that Chrome writes crash dumps while not logged in",
		SoftwareDeps: []string{"chrome"},
	})
}

func ChromeCrashNotLoggedIn(s *testing.State) {
	defer faillog.SaveIfError(s)

	cr, err := chrome.New(s.Context(), chrome.NoLogin())
	if err != nil {
		s.Fatal("Chrome startup failed: ", err)
	}
	defer cr.Close(s.Context())

	if dumps, err := chromecrash.KillAndGetDumps(s.Context()); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	} else if len(dumps) == 0 {
		s.Error("No minidumps written after not-logged-in Chrome crash")
	}
}
