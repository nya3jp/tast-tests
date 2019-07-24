// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     TwoShills,
		Desc:     "Verifies that an attempt to spawn a second instance of shill while an instance is already running will fail",
		Contacts: []string{"billyzhao@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:     []string{"informational"},
	})
}

func TwoShills(ctx context.Context, s *testing.State) {
	const timeout time.Duration = 10
	_, _, pid, err := upstart.JobStatus(ctx, "shill")
	if err != nil {
		s.Fatal("Failed to find shill job: ", err)
	} else if pid == 0 {
		s.Fatal("Shill is not running")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout*time.Second)
	defer cancel()
	if err := testexec.CommandContext(ctx, "shill", "--foreground").Run(testexec.DumpLogOnError); err != nil {
		s.Log("Shill errored with with reason: ", err)
		if ctx.Err() == context.DeadlineExceeded {
			s.Fatal("Second shill started but didn't exit before timeout: ", ctx.Err())
		}
	} else {
		s.Fatal("Invocation of second shill instance should have failed")
	}
}
