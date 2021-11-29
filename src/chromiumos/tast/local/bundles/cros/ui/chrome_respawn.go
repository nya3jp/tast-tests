// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeRespawn,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Chrome respawns after exit",
		Contacts: []string{
			"mnissler@chromium.org", // session_manager owner
			"chromeos-ui@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func ChromeRespawn(ctx context.Context, s *testing.State) {
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure ui job is running: ", err)
	}

	// Chrome process should be running while ui job is running.
	proc, err := ashproc.WaitForRoot(ctx, 30*time.Second)
	if err != nil {
		s.Fatal("Failed to getting initial chrome process: ", err)
	}
	s.Logf("Initial chrome process is %d", proc.Pid)

	// Forcibly terminate the chrome.
	if err := proc.Kill(); err != nil {
		s.Fatal("Failed to kill chrome: ", err)
	}
	if err := procutil.WaitForTerminated(ctx, proc, 10*time.Second); err != nil {
		s.Fatal("Old chrome process was not terminated: ", err)
	}

	// New Chrome should be automatically respawned.
	newProc, err := ashproc.WaitForRoot(ctx, 30*time.Second)
	if err != nil {
		s.Fatal("Failed waiting for chrome to respawn: ", err)
	}
	s.Logf("Respawned chrome process is %d", newProc.Pid)
}
