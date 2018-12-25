// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package upstart interacts with the Upstart init daemon on behalf of local tests.
package upstart

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Goal describes a job's goal. See Section 10.1.6.19, "initctl status", in the Upstart Cookbook.
type Goal string

// State describes a job's current state. See Section 4.1.2, "Job States", in the Upstart Cookbook.
type State string

const (
	// StartGoal indicates that a task or service job has been started.
	StartGoal Goal = "start"
	// StopGoal indicates that a task job has completed or that a service job has been manually stopped or has
	// a "stop on" condition that has been satisfied.
	StopGoal Goal = "stop"

	// WaitingState is the initial state for a job.
	WaitingState State = "waiting"
	// StartingState indicates that a job is about to start.
	StartingState State = "starting"
	// SecurityState indicates that a job is having its AppArmor security policy loaded.
	SecurityState State = "security"
	// PreStartState indicates that a job's pre-start section is running.
	PreStartState State = "pre-start"
	// SpawnedState indicates that a job's script or exec section is about to run.
	SpawnedState State = "spawned"
	// PostStartState indicates that a job's post-start section is running.
	PostStartState State = "post-start"
	// RunningState indicates that a job is running (i.e. its post-start section has completed). It may not have a PID yet.
	RunningState State = "running"
	// PreStopState indicates that a job's pre-stop section is running.
	PreStopState State = "pre-stop"
	// StoppingState indicates that a job's pre-stop section has completed.
	StoppingState State = "stopping"
	// KilledState indicates that a job is about to be stopped.
	KilledState State = "killed"
	// PostStopState indicates that a job's post-stop section is running.
	PostStopState State = "post-stop"

	uiJob = "ui" // special-cased in StopJob due to its respawning behavior
)

var statusRegexp *regexp.Regexp
var allGoals map[Goal]struct{}
var allStates map[State]struct{}

func init() {
	// Matches a leading line of e.g. "ui start/running, process 3182" or "boot-splash stop/waiting".
	statusRegexp = regexp.MustCompile(`(?m)^[^ ]+ ([-a-z]+)/([-a-z]+)(?:, process (\d+))?$`)

	allGoals = map[Goal]struct{}{StartGoal: struct{}{}, StopGoal: struct{}{}}

	allStates = make(map[State]struct{})
	for _, s := range []State{WaitingState, StartingState, SecurityState, PreStartState, SpawnedState,
		PostStartState, RunningState, PreStopState, StoppingState, KilledState, PostStopState} {
		allStates[s] = struct{}{}
	}
}

// JobStatus returns the current status of job.
// If the PID is unavailable (i.e. the process is not running), 0 will be returned.
// An error will be returned if the job is unknown (i.e. it has no config in /etc/init).
func JobStatus(ctx context.Context, job string) (goal Goal, state State, pid int, err error) {
	c := testexec.CommandContext(ctx, "initctl", "status", job)
	b, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		return goal, state, pid, err
	}
	return parseStatus(job, string(b))
}

// parseStatus parses the output from "initctl status <job>", e.g. "ui start/running, process 28515".
// The output may be multiple lines; see the example in Section 10.1.6.19.3,
// "Single Job Instance Running with Multiple PIDs", in the Upstart Cookbook.
func parseStatus(job, out string) (goal Goal, state State, pid int, err error) {
	if !strings.HasPrefix(out, job+" ") {
		return goal, state, pid, errors.Errorf("missing job prefix %q in %q", job, out)
	}
	m := statusRegexp.FindStringSubmatch(out)
	if m == nil {
		return goal, state, pid, errors.Errorf("unexpected format in %q", out)
	}

	goal = Goal(m[1])
	if _, ok := allGoals[goal]; !ok {
		return goal, state, pid, errors.Errorf("invalid goal %q", m[1])
	}

	state = State(m[2])
	if _, ok := allStates[state]; !ok {
		return goal, state, pid, errors.Errorf("invalid state %q", m[2])
	}

	if m[3] != "" {
		p, err := strconv.ParseInt(m[3], 10, 32)
		if err != nil {
			return goal, state, pid, errors.Errorf("bad PID %q", m[3])
		}
		pid = int(p)
	}

	return goal, state, pid, nil
}

// JobExists returns true if the supplied job exists (i.e. it has a config file known by Upstart).
func JobExists(ctx context.Context, job string) bool {
	if err := testexec.CommandContext(ctx, "initctl", "status", job).Run(); err != nil {
		return false
	}
	return true
}

// RestartJob restarts job. If the job is currently stopped, it will be started.
// Note that the job is reloaded if it is already running; this differs from the
// "initctl restart" behavior as described in Section 10.1.2, "restart", in the Upstart Cookbook.
// args is passed to the job as extra parameters.
func RestartJob(ctx context.Context, job string, args ...string) error {
	// Make sure that the job isn't running and then try to start it.
	if err := StopJob(ctx, job); err != nil {
		return errors.Wrapf(err, "stopping %q failed", job)
	}
	if err := StartJob(ctx, job, args...); err != nil {
		return errors.Wrapf(err, "starting %q failed", job)
	}
	return nil
}

// StopJob stops job. If it is not currently running, this is a no-op.
//
// The ui job receives special behavior since it is restarted out-of-band by the ui-respawn
// job when session_manager exits. To work around this, when job is "ui", this function first
// waits for the job to reach a stable state. See https://crbug.com/891594.
func StopJob(ctx context.Context, job string) error {
	if job == uiJob {
		// The ui and ui-respawn jobs go through the following sequence of statuses
		// when the session_manager job exits with a nonzero status:
		//
		//	a) ui start/running, process 29325    ui-respawn stop/waiting
		//	b) ui stop/stopping                   ui-respawn start/running, process 30567
		//	c) ui start/post-stop, process 30586  ui-respawn stop/waiting
		//	d) ui start/starting                  ui-respawn stop/waiting
		//	e) ui start/pre-start, process 30935  ui-respawn stop/waiting
		//	f) ui start/running, process 30946    ui-respawn stop/waiting
		//
		// Run "initctl stop" first to ensure that waitUIJobStabilized doesn't see a).
		// It's possible that this command will fail if it's run during b), but in that case
		// waitUIJobStabilized should wait for the ui job to return to c), in which case the
		// following "initctl stop" command should succeed in bringing the job back to "stop/waiting".
		testexec.CommandContext(ctx, "initctl", "stop", job).Run()
		if err := waitUIJobStabilized(ctx); err != nil {
			return errors.Wrapf(err, "failed waiting for %v job to stabilize", job)
		}
	}

	// Issue a "stop" request and hope for the best.
	cmd := testexec.CommandContext(ctx, "initctl", "stop", job)
	cmdErr := cmd.Run()

	// If the job was already stopped, the above "initctl stop" would have failed.
	// Check its actual status now.
	if err := WaitForJobStatus(ctx, job, StopGoal, WaitingState, RejectWrongGoal, 0); err != nil {
		if cmdErr != nil {
			cmd.DumpLog(ctx)
		}
		return err
	}
	return nil
}

// waitUIJobStabilized is a helper function for StopJob that waits for the ui
// job to either have a "start" goal or reach "stop/waiting" while the ui-respawn job
// is in "stop/waiting".
func waitUIJobStabilized(ctx context.Context) error {
	const (
		respawnJob = "ui-respawn"
		timeout    = 30 * time.Second
	)

	return testing.Poll(ctx, func(ctx context.Context) error {
		ug, us, _, _ := JobStatus(ctx, uiJob)
		uiStable := ug == StartGoal || (ug == StopGoal && us == WaitingState)

		rg, rs, _, _ := JobStatus(ctx, respawnJob)
		respawnStopped := rg == StopGoal && rs == WaitingState

		if !uiStable || !respawnStopped {
			return errors.Errorf("%v status %v/%v, %v status %v/%v", uiJob, ug, us, respawnJob, rg, rs)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// EnsureJobRunning starts job if it isn't currently running.
// If it is already running, this is a no-op.
func EnsureJobRunning(ctx context.Context, job string) error {
	// If the job already has a "start" goal, wait for it to enter the "running" state.
	// This will return nil immediately if it's already start/running, and will return
	// an error immediately if the job has a "stop" goal.
	if err := WaitForJobStatus(ctx, job, StartGoal, RunningState, RejectWrongGoal, 0); err == nil {
		return nil
	}

	// Otherwise, start it. This command blocks until the job enters the "running" state.
	return StartJob(ctx, job)
}

// StartJob starts job. If it is already running, this returns an error.
// args is passed to the job as extra parameters.
func StartJob(ctx context.Context, job string, args ...string) error {
	cmd := testexec.CommandContext(ctx, "initctl", append([]string{"start", job}, args...)...)
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// GoalPolicy describes how WaitForJobStatus should handle mismatched goals.
type GoalPolicy int

const (
	// TolerateWrongGoal indicates that it's acceptable for the job to initially
	// have a goal that doesn't match the requested one. WaitForJobStatus will
	// continue waiting for the requested goal.
	TolerateWrongGoal GoalPolicy = iota
	// RejectWrongGoal indicates that an error should be returned immediately
	// if the job doesn't have the requested goal.
	RejectWrongGoal
)

// WaitForJobStatus waits for job to have the status described by goal/state.
// gp controls the function's behavior if the job's goal doesn't match the requested one.
// If timeout is non-zero, it limits the amount of time to wait.
func WaitForJobStatus(ctx context.Context, job string, goal Goal, state State, gp GoalPolicy,
	timeout time.Duration) error {
	// Used to report an out-of-band error if we fail to get the status or see a different goal.
	var statusErr error
	err := testing.Poll(ctx, func(ctx context.Context) error {
		g, s, _, err := JobStatus(ctx, job)
		if err != nil {
			statusErr = err
			return nil
		}
		if g != goal && gp == RejectWrongGoal {
			statusErr = errors.Errorf("status %v/%v has non-%q goal", g, s, goal)
			return nil
		}
		if g != goal || s != state {
			return errors.Errorf("status %v/%v", g, s)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})

	if statusErr != nil {
		return statusErr
	}
	return err
}
