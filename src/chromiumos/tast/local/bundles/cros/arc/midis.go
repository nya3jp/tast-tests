// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Midis,
		Desc: "Verifies midis daemon starts correctly",
		Contacts: []string{
			"pmalani@chromium.org", // original author
			"arc-eng@google.com",
			"hidehiko@chromium.org", // Tast port author
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

func Midis(ctx context.Context, s *testing.State) {
	type expect int
	const (
		midisExe = "/usr/bin/midis"

		running expect = iota
		stopped
	)

	waitForMidis := func(ctx context.Context, e expect) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			all, err := process.Pids()
			if err != nil {
				return testing.PollBreak(err)
			}
			for _, pid := range all {
				p, err := process.NewProcess(int32(pid))
				if err != nil {
					// Process is terminated after listing all PIDs.
					// Skip here.
					continue
				}

				exe, err := p.Exe()
				if err != nil {
					// As same as above, process may be terminated.
					continue
				}
				if exe == midisExe {
					if e == stopped {
						// Midis is still running. Wait for the next cycle.
						return errors.New("midis is unexpectly running")
					}
					return nil
				}
			}
			if e == running {
				// Midis is not yet started. Wait for the next cycle.
				return errors.New("midis is not running")
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second})
	}

	// Ensure login screen.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui: ", err)
	}
	if err := waitForMidis(ctx, stopped); err != nil {
		s.Fatal("Midis should not running in login screen: ", err)
	}

	// Log in to Chrome, and verify midis is running.
	func() {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to log in Chrome: ", err)
		}
		defer cr.Close(ctx)

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()

		if err := waitForMidis(ctx, running); err != nil {
			s.Fatal("Midis should run: ", err)
		}
	}()

	// Log out from Chrome.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out from Chrome: ", err)
	}
	if err := waitForMidis(ctx, stopped); err != nil {
		s.Fatal("Midis does not stop on Chrome logout: ", err)
	}
}
