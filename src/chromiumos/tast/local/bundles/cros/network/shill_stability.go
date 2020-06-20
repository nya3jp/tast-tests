// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillStability,
		Desc: "Checks that shill isn't respawning",
		Contacts: []string{
			"npoojary@chromium.org",    // Connectivity team
			"briannorris@chromium.org", // Tast port author
		},
		Attr: []string{"group:mainline"},
	})
}

func ShillStability(ctx context.Context, s *testing.State) {
	const (
		stabilityInterval = time.Second
		stabilityDuration = 20 * time.Second
	)

	// Returns PID of main shill process. Calls s.Fatal if we can't find
	// shill.
	getPID := func() int {
		_, _, pid, err := upstart.JobStatus(ctx, "shill")
		if err != nil {
			s.Fatal("Failed to find shill job: ", err)
		} else if pid == 0 {
			s.Fatal("Shill is not running")
		}
		return pid
	}

	pid := getPID()

	testing.Poll(ctx, func(ctx context.Context) error {
		if latest := getPID(); latest != pid {
			s.Fatalf("PID changed: %v -> %v", pid, latest)
		}
		s.Log("PID stable at: ", pid)
		return errors.New("keep polling")
	}, &testing.PollOptions{Timeout: stabilityDuration, Interval: stabilityInterval})
}
