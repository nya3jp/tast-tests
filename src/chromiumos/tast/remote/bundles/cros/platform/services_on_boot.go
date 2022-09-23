// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/common/upstart"
	"chromiumos/tast/remote/bundles/cros/platform/fixtures"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServicesOnBoot,
		Desc:         "Check services are started successfully on boot",
		SoftwareDeps: []string{"reboot"},
		Contacts:     []string{"aaronyu@google.com", "chromeos-audio-sw@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixtures.ServicesOnBootFixt,
		ServiceDeps:  []string{"tast.cros.platform.UpstartService"},
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			// The early-failure service always fails.
			// Use this to check the fixture is collecting logs correctly.
			{
				Name: "smoke",
				Val:  checkFailed("early-failure", 124),
			},
			// Audio services.
			// Contacts: aaronyu@google.com, chromeos-audio-sw@google.com.
			{
				Name:              "cras",
				Val:               chcekStatus("cras", upstart.StartGoal, upstart.RunningState),
				ExtraSoftwareDeps: []string{"cras"},
			},
			{
				Name:              "sound_card_init",
				Val:               chcekStatus("sound_card_init", upstart.StopGoal, upstart.WaitingState),
				ExtraSoftwareDeps: []string{"cras"},
			},
		},
	})
}

// ServicesOnBoot checks that services are at their desired state on boot.
//
// NOTE: It's not named BootServices because "boot-services" has special
// meaning in ChromeOS's boot flow.
func ServicesOnBoot(ctx context.Context, s *testing.State) {
	fixt := s.FixtValue().(*fixtures.ServicesOnBootFixtVal)
	checker := s.Param().(serviceChecker)

	checker(ctx, s, fixt)
}

// serviceChecker checks for each a service's status after boot.
//
// It takes testing.State to make reporting multiple errors easier.
type serviceChecker func(context.Context, *testing.State, *fixtures.ServicesOnBootFixtVal)

// chcekStatus returns a serviceChecker which checks that the job did not fail
// and is in the desired goal and state.
func chcekStatus(jobName string, goal upstart.Goal, state upstart.State) serviceChecker {
	return func(ctx context.Context, s *testing.State, fixt *fixtures.ServicesOnBootFixtVal) {
		// Check that there are no problems when starting the job.
		//
		// A job may start with multiple retries.
		// Here we treat any retry as an error to catch race conditions in service dependencies.
		for _, failure := range fixt.Failures {
			if failure.JobName == jobName {
				s.Errorf("Job %s failed: %s", jobName, failure.Message)
			}
		}

		// Check for "started/running".
		status, err := fixt.Upstart.JobStatus(ctx, &platform.JobStatusRequest{
			JobName: jobName,
		})
		if err != nil {
			s.Fatalf("Failed to get job %s status: %s", jobName, err)
		}
		if status.Goal != string(goal) || status.State != string(state) {
			s.Fatalf("Job %s not in expected state; want: %s/%s, got: %s/%s",
				jobName,
				goal, state,
				status.Goal, status.State,
			)
		}
	}
}

// checkFailed returns a serviceChecker which checks that the job exited with exitStatus
func checkFailed(jobName string, exitStatus int) serviceChecker {
	return func(ctx context.Context, s *testing.State, fixt *fixtures.ServicesOnBootFixtVal) {
		for _, failure := range fixt.Failures {
			if failure.JobName == jobName && failure.ExitStatus == exitStatus {
				return
			}
		}
		s.Errorf("Job %s did not terminate with %d", jobName, exitStatus)
	}
}
