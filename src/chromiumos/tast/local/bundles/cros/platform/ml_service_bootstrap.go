// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MLServiceBootstrap,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Chrome can establish a Mojo connection to ML Service",
		Contacts:     []string{"amoylan@chromium.org"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "ml_service"},
		Pre:          chrome.LoggedIn(),
	})
}

func MLServiceBootstrap(ctx context.Context, s *testing.State) {
	const (
		instanceParameter = "TASK"
		instance          = "mojo_service"
	)
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const job = "ml-service"

	s.Log("Stopping ML Service daemon if it is running")
	if err = upstart.StopJob(ctx, job, upstart.WithArg(instanceParameter, instance)); err != nil {
		s.Fatalf("Failed to stop %s: %v", job, err)
	}

	s.Log("Waiting for ML Service daemon to fully stop")
	if err := upstart.WaitForJobStatus(ctx, job, upstartcommon.StopGoal, upstartcommon.WaitingState, upstart.RejectWrongGoal, 15*time.Second, upstart.WithArg(instanceParameter, instance)); err != nil {
		s.Fatalf("Failed waiting for %v to stop: %v", job, err)
	}

	s.Log("Waiting for Chrome to complete a basic call to ML Service")
	if err = tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.bootstrapMachineLearningService)`); err != nil {
		s.Fatal("Running autotestPrivate.bootstrapMachineLearningService failed: ", err)
	}

	s.Log("Checking ML Service is running")
	if err := upstart.WaitForJobStatus(ctx, job, upstartcommon.StartGoal, upstartcommon.RunningState, upstart.RejectWrongGoal, 15*time.Second, upstart.WithArg(instanceParameter, instance)); err != nil {
		s.Fatalf("Failed waiting for %v to start: %v", job, err)
	}
}
