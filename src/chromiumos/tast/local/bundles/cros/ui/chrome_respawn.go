// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/bundles/cros/ui/respawn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeRespawn,
		Desc:         "Checks that Chrome respawns after exit",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChromeRespawn(s *testing.State) {
	if err := upstart.EnsureJobRunning(s.Context(), "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
	respawn.TestRespawn(s, "Chrome", chrome.GetRootPID)
}
