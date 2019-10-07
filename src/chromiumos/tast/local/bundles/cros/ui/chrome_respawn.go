// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/respawn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeRespawn,
		Desc: "Checks that Chrome respawns after exit",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"chromeos-ui@google.com",
		},
		SoftwareDeps: []string{"chrome"},
	})
}

func ChromeRespawn(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}
	respawn.TestRespawn(ctx, s, "Chrome", chrome.GetRootPID)
}
