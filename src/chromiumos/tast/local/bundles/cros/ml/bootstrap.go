// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ml

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Bootstrap,
		Desc:         "Checks that Chrome can establish a Mojo connection to ML Service",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func Bootstrap(s *testing.State) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

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
		       reject(new Error(chrome.runtime.lastError.message));
		     }
		   });
		 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.bootstrapMachineLearningService failed: ", err)
	}

	s.Log("Checking ML Service is running")
	if running, _, err := upstart.JobStatus(ctx, job); err != nil {
		s.Fatalf("Failed to get status of job %s: %v", job, err)
	} else if !running {
		s.Fatalf("Job %s not running")
	}
}
