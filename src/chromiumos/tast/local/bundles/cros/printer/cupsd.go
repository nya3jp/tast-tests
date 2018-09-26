// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/local/systemd"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CUPSD,
		Desc:         "Sanity test for cupsd and the upstart-socket-bridge socket-activation",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"printer"},
	})
}

func CUPSD(s *testing.State) {
	const (
		sockPath = "/run/cups/cups.sock"
	)

	// Checks if systemd is used.
	{
		enabled, err := systemd.Enabled()
		if err != nil {
			s.Fatal("Failed to check if systemd is used: ", err)
		}
		if enabled {
			// TODO(crbug.com/889488): Currently tast is not
			// supported on a device with systemd.
			s.Fatal("systemd device is not supported")
		}
	}

	ctx := s.Context()
	// Check if CUPS is operating.
	isRunning := func(ctx context.Context) error {
		// Try a simple CUPS command; failed if it takes too long
		// (i.e., socket may exist, but it may not get passed off
		// to cupsd properly).
		ectx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		cmd := testexec.CommandContext(ectx, "lpstat", "-W", "all")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx) // Ignore the error of DumpLog.
			return err
		}
		return nil
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

	if err := isRunning(ctx); err != nil {
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
	f, err := os.OpenFile(sockPath, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		s.Fatal("Failed to create a dummy socket: ", err)
	}
	if err = f.Close(); err != nil {
		s.Fatal("Failed to touch a dummy socket: ", err)
	}

	if err = upstart.RestartJob(ctx, "upstart-socket-bridge"); err != nil {
		s.Fatal("Failed to start upstart-socket-bridge: ", err)
	}
	if _, err = os.Stat(sockPath); err != nil {
		s.Fatal("Missing CUPS socket: ", err)
	}

	if err = isRunning(ctx); err != nil {
		s.Fatal("CUPS is not operating properly: ", err)
	}
}
