// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/bundles/cros/ui/chromecrash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeCrashLoggedIn,
		Desc: "Checks that Chrome writes crash dumps while logged in",
		Pre:  chrome.LoggedIn(),
	})
}

func ChromeCrashLoggedIn(s *testing.State) {
	if dumps, err := chromecrash.KillAndGetDumps(s.Context()); err != nil {
		s.Fatal("Couldn't kill Chrome or get dumps: ", err)
	} else if len(dumps) == 0 {
		s.Error("No minidumps written after logged-in Chrome crash")
	}
}
