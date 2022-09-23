// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/testexec"
	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ServicesOnBoot,
		Desc:         "Check services are started successfully on boot",
		SoftwareDeps: []string{"reboot"},
		Contacts:     []string{"aaronyu@google.com", "chromeos-audio-sw@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ServicesOnBoot,
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			// The early-failure service always fails.
			// Use this to check the fixture is collecting logs correctly.
			{
				Name: "smoke",
				Val:  checkFailed("early-failure", 124),
			},
			// All services.
			// This allows us to catch failures in newly added services.
			// Over time we hope to stabilize this test.
			{
				Name: "all",
				Val: checkAll(&checkAllOpt{
					excludeServices: []string{
						"early-failure",           // always fails
						"cras", "sound_card_init", // these have separate tests
					},
				}),
			},
			// Audio services.
			// Contacts: aaronyu@google.com, chromeos-audio-sw@google.com.
			{
				Name:              "cras",
				Val:               checkStatus("cras", upstartcommon.StartGoal, upstartcommon.RunningState),
				ExtraSoftwareDeps: []string{"cras"},
			},
			{
				Name:              "sound_card_init",
				Val:               checkStatus("sound_card_init", upstartcommon.StopGoal, upstartcommon.WaitingState),
				ExtraSoftwareDeps: []string{"cras"},
			},
		},
	})
}

// ServicesOnBoot checks that services are at their desired state on boot.
//
// The test is organized into a fixture that is reserved for this test and parameterized tests:
//  1. A reboot is needed to avoid events happen before the test is run to
//     interfere with the test results.
//  2. Parameterized tests are used to report test results of different services separately.
//  3. A Fixture is used so that each parameterized test does not need to perform a reboot.
//  4. The fixture shall not be used by other tests, so that the other tests do not
//     alter the service statuses.
//
// NOTE: It's not named BootServices because "boot-services" has special
// meaning in ChromeOS's boot flow.
func ServicesOnBoot(ctx context.Context, s *testing.State) {
	checker := s.Param().(serviceChecker)

	checker(ctx, s)
}

// serviceChecker checks for each a service's status after boot.
//
// It takes testing.State to make reporting multiple errors easier.
type serviceChecker func(context.Context, *testing.State)

// checkStatus returns a serviceChecker which checks that the job did not fail
// and is in the desired goal and state.
func checkStatus(jobName string, wantGoal upstartcommon.Goal, wantState upstartcommon.State) serviceChecker {
	return func(ctx context.Context, s *testing.State) {
		failures, err := getServiceFailures(ctx)
		if err != nil {
			s.Fatal("Cannot get service failures: ", err)
		}

		// Check that there are no problems when starting the job.
		//
		// A job may start with multiple retries.
		// Here we treat any retry as an error to catch race conditions in service dependencies.
		for _, failure := range failures {
			if failure.JobName == jobName {
				s.Errorf("Job %s failed: %s", jobName, failure.Message)
			}
		}

		// Check for job status.
		gotGoal, gotState, _, err := upstart.JobStatus(ctx, jobName)
		if err != nil {
			s.Fatalf("Failed to get job %s status: %s", jobName, err)
		}
		if gotGoal != wantGoal || gotState != wantState {
			s.Fatalf("Job %s not in expected state; want: %s/%s, got: %s/%s",
				jobName,
				wantGoal, wantState,
				gotGoal, gotState,
			)
		}
	}
}

// checkFailed returns a serviceChecker which checks that the job exited with exitStatus.
func checkFailed(jobName string, exitStatus int) serviceChecker {
	return func(ctx context.Context, s *testing.State) {
		failures, err := getServiceFailures(ctx)
		if err != nil {
			s.Fatal("Cannot get service failures: ", err)
		}

		for _, failure := range failures {
			if failure.JobName == jobName && failure.ExitStatus == exitStatus {
				return
			}
		}
		s.Errorf("Job %s did not terminate with %d", jobName, exitStatus)
	}
}

type checkAllOpt struct {
	excludeServices []string // upstart job names to exclude
}

// checkAll returns a serviceChecker which checks that no services failed.
func checkAll(opt *checkAllOpt) serviceChecker {
	excludeServices := make(map[string]bool)
	for _, jobName := range opt.excludeServices {
		excludeServices[jobName] = true
	}

	return func(ctx context.Context, s *testing.State) {
		failures, err := getServiceFailures(ctx)
		if err != nil {
			s.Fatal("Cannot get service failures: ", err)
		}

		for _, failure := range failures {
			if !excludeServices[failure.JobName] {
				s.Errorf("Job %s failed: %s", failure.JobName, failure.Message)
			}
		}
	}
}

// serviceFailure is an error log entry from init.
type serviceFailure struct {
	Message        string
	JobName        string
	ProcessName    string
	ExitStatus     int // The exit status of the job. -1 means abnormal exit.
	KilledBySignal string
}

func parseServiceFailures(dmesg string) ([]serviceFailure, error) {
	// Regular expression for upstart logs:
	//	[    1.374461] init: early-failure main process (315) terminated with status 124
	//	[    9.131824] init: failsafe-delay main process (944) killed by TERM signal
	initRE := regexp.MustCompile(`\[[0-9\. ]+\] init: (\S+) (\S+) process \(\d+\) (?:terminated with status (\d+)|killed by (\S+) signal)`)

	messages := initRE.FindAllStringSubmatch(string(dmesg), -1)
	result := make([]serviceFailure, 0, len(messages))
	for _, message := range messages {
		failure := serviceFailure{
			Message:     message[0],
			JobName:     message[1],
			ProcessName: message[2],
		}
		if message[3] != "" {
			var err error
			failure.ExitStatus, err = strconv.Atoi(message[3])
			if err != nil {
				return nil, errors.Errorf("cannot parse exit status of %q", message[0])
			}
		} else {
			failure.ExitStatus = -1
			failure.KilledBySignal = message[4]
		}
		result = append(result, failure)
	}
	return result, nil
}

func getServiceFailures(ctx context.Context) ([]serviceFailure, error) {
	cmd := testexec.CommandContext(ctx,
		"croslog", "--boot=0", "--identifier=kernel",
	)
	stdout, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Errorf("failed to get kernel messages: %s", err)
	}
	return parseServiceFailures(string(stdout))
}
