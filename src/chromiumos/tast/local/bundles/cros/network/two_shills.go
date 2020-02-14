// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TwoShills,
		Desc:     "Verifies that an attempt to spawn a second instance of shill while an instance is already running will fail",
		Contacts: []string{"deanliao@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline"},
	})
}

func TwoShills(ctx context.Context, s *testing.State) {
	if _, _, pid, err := upstart.JobStatus(ctx, "shill"); err != nil {
		s.Fatal("Failed to find shill job: ", err)
	} else if pid == 0 {
		s.Fatal("Shill is not running")
	}

	// Make sure Shill is not only running, but that it's listening on
	// D-Bus. Note that Shill could be restarting from a previous test, so
	// we could have a race condition (e.g., where we acquire the Shill
	// D-Bus namespace before the system Shill does).
	if _, err := shill.NewManager(ctx); err != nil {
		s.Fatal("Failed creating Manager proxy: ", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := testexec.CommandContext(ctx, "shill", "--foreground").Run(testexec.DumpLogOnError); err != nil {
		s.Log("Shill errored with reason: ", err)
		if err == ctx.Err() {
			s.Fatal("Second shill started but didn't exit before timeout: ", ctx.Err())
		}
	} else {
		s.Fatal("Invocation of second shill instance should have failed")
	}
}
