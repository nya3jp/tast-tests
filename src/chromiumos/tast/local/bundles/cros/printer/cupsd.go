// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printer"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CUPSD,
		Desc: "Sanity test for cupsd and the upstart-socket-bridge socket-activation",
		Contacts: []string{
			"briannorris@chromium.org", // Original autotest author
			"hidehiko@chromium.org",    // Tast port author
		},
		SoftwareDeps: []string{"chrome_login", "cups"},
		Pre:          chrome.LoggedIn(),
	})
}

func CUPSD(ctx context.Context, s *testing.State) {
	const sockPath = "/run/cups/cups.sock"

	// At the end of testing, restore the default upstart job state.
	// "upstart-socket-bridge" is expected to be running.
	// "cupsd" is stopped.
	defer upstart.RestartJob(ctx, "upstart-socket-bridge")
	defer printer.ResetCups(ctx)

	// Check if CUPS is operating.
	isRunning := func() error {
		// Try a simple CUPS command; failed if it takes too long
		// (i.e., socket may exist, but it may not get passed off
		// to cupsd properly).
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := testexec.CommandContext(ctx, "lpstat", "-W", "all")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx) // Ignore the error of DumpLog.
			return err
		}
		return nil
	}

	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}

	if err := upstart.EnsureJobRunning(ctx, "upstart-socket-bridge"); err != nil {
		s.Fatal("upstart-socket-bridge is not running: ", err)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		_, err := os.Stat(sockPath)
		return err
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		s.Fatal("Missing CUPS socket: ", err)
	}

	// Make sure CUPS is stopped, so we can test on-demand launch.
	if err := upstart.StopJob(ctx, "cupsd"); err != nil {
		s.Fatal("Failed to ensure stopping cupsd: ", err)
	}

	if err := isRunning(); err != nil {
		s.Fatal("CUPS is not operating properly: ", err)
	}

	// Now try stopping socket bridge, to see it clean up its files.
	if err := upstart.StopJob(ctx, "upstart-socket-bridge"); err != nil {
		s.Fatal("Failed to stop upstart-socket-bridge: ", err)
	}
	if err := upstart.StopJob(ctx, "cupsd"); err != nil {
		s.Fatal("Failed to stop cupsd: ", err)
	}
	if _, err := os.Stat(sockPath); err == nil || !os.IsNotExist(err) {
		s.Fatal("CUPS socket was not cleaned up: ", err)
	}

	// Create dummy file, to see if upstart-socket-bridge will clear it out
	// properly.
	if err := ioutil.WriteFile(sockPath, nil, 0666); err != nil {
		s.Fatal("Failed to create a dummy socket: ", err)
	}

	if err := upstart.RestartJob(ctx, "upstart-socket-bridge"); err != nil {
		s.Fatal("Failed to start upstart-socket-bridge: ", err)
	}
	if _, err := os.Stat(sockPath); err != nil {
		s.Fatal("Missing CUPS socket: ", err)
	}

	if err := isRunning(); err != nil {
		s.Fatal("CUPS is not operating properly: ", err)
	}
}
