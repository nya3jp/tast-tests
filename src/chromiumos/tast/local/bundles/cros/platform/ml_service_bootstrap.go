// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MLServiceBootstrap,
		Desc:         "Checks that Chrome can establish a Mojo connection to ML Service",
		Contacts:     []string{"amoylan@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "ml_service"},
		Pre:          chrome.LoggedIn(),
	})
}

func MLServiceBootstrap(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}

	const job = "ml-service"

	s.Log("Stopping ML Service daemon if it is running")
	if err = upstart.StopJob(ctx, job); err != nil {
		s.Fatalf("Failed to stop %s: %v", job, err)
	}

	s.Log("Waiting for Chrome to complete a basic call to ML Service")
	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.bootstrapMachineLearningService(() => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(chrome.runtime.lastError.message);
		     }
		   });
		 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.bootstrapMachineLearningService failed: ", err)
	}

	s.Log("Checking ML Service is running")
	if goal, state, _, err := upstart.JobStatus(ctx, job); err != nil {
		s.Fatalf("Failed to get status of job %s: %v", job, err)
	} else if goal != upstart.StartGoal || state != upstart.RunningState {
		s.Fatalf("Job %s is %s/%s; want %s/%s", job, goal, state, upstart.StartGoal, upstart.RunningState)
	}
}
