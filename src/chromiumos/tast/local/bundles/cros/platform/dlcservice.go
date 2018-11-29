// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DLCService,
		Desc: "Launches dlcservice to verify dlcservice exits on idle and can accept API calls",
		Attr: []string{"informational"},
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		job = "dlcservice"
	)

	// dlcservice is a short-lived process.
	// Restarts dlcservice and checks if it exits on idle.
	s.Logf("Restarting %s job", job)
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatalf("Failed to start %s: %v", job, err)
	}

	if err := upstart.WaitForJobStatus(ctx, job, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", job, err)
	}
	s.Logf("Job %s stopped", job)

	// dlcservice is activated on-demand via D-Bus method call.
	// Calls dlcservice's GetInstalled D-Bus method, checks the return results, and checks if it exits on idle.
	s.Logf("Asking dlcservice for installed DLC modules")
	cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
	if out, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to get installed DLC modules")
	} else {
		s.Logf("Installed DLC modules: %s", out)
	}

	if err := upstart.WaitForJobStatus(ctx, job, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", job, err)
	}
	s.Logf("Job %s stopped", job)
}
