// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package upstart interacts with the Upstart init daemon on behalf of local tests.
package upstart

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Goal describes a job's goal. See Section 10.1.6.19, "initctl status" in the Upstart Cookbook.
type Goal string

// State describes a job's current state. See Section 4.1.2, "Job States" in the Upstart Cookbook.
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
)

var statusRegexp *regexp.Regexp
var allGoals map[Goal]struct{}
var allStates map[State]struct{}

func init() {
	// Matches e.g. "ui start/running, process 3182" or "boot-splash stop/waiting".
	statusRegexp = regexp.MustCompile("^[^ ]+ ([-a-z]+)/([-a-z]+)(?:, process (\\d+))?$")

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
	return parseStatus(job, strings.TrimSpace(string(b)))
}

// parseStatus parses the output from "initctl status <job>", e.g. "ui start/running, process 28515".
func parseStatus(job, line string) (goal Goal, state State, pid int, err error) {
	if !strings.HasPrefix(line, job+" ") {
		return goal, state, pid, fmt.Errorf("missing job prefix %q in %q", job, line)
	}
	m := statusRegexp.FindStringSubmatch(line)
	if m == nil {
		return goal, state, pid, fmt.Errorf("unexpected format in %q", line)
	}

	goal = Goal(m[1])
	if _, ok := allGoals[goal]; !ok {
		return goal, state, pid, fmt.Errorf("invalid goal %q", m[1])
	}

	state = State(m[2])
	if _, ok := allStates[state]; !ok {
		return goal, state, pid, fmt.Errorf("invalid state %q", m[2])
	}

	if m[3] != "" {
		p, err := strconv.ParseInt(m[3], 10, 32)
		if err != nil {
			return goal, state, pid, fmt.Errorf("bad PID %q", m[3])
		}
		pid = int(p)
	}

	return goal, state, pid, nil
}

// RestartJob restarts job. If the job is currently stopped, it will be started.
// If the job is already running, its configuration will not be reloaded and its post-stop,
// pre-start, and post-start stanzas will not run (but pre-stop will run).
// See Section 10.1.2, "restart", in the Upstart Cookbook for details.
func RestartJob(ctx context.Context, job string) error {
	// If the job currently has a "start" goal, this will restart it.
	// If it has a "stop" goal, this will fail and we'll need to start it below instead.
	cmd := testexec.CommandContext(ctx, "initctl", "restart", job)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Make sure that the job isn't running and then try to start it.
	if err := StopJob(ctx, job); err != nil {
		return fmt.Errorf("stopping %q failed: %v", job, err)
	}
	if err := EnsureJobRunning(ctx, job); err != nil {
		return fmt.Errorf("starting %q failed: %v", job, err)
	}
	return nil
}

// StopJob stops job. If it is not currently running, this is a no-op.
func StopJob(ctx context.Context, job string) error {
	// Issue a "stop" request and hope for the best.
	cmd := testexec.CommandContext(ctx, "initctl", "stop", job)
	cmdErr := cmd.Run()

	// If the job was already stopped, the above "initctl stop" would have failed.
	// Check its actual status now.
	if err := WaitForJobStatus(ctx, job, StopGoal, WaitingState, 0); err != nil {
		if cmdErr != nil {
			testing.ContextLogf(ctx, "%q failed: %v", strings.Join(cmd.Args, " "), cmdErr)
			cmd.DumpLog(ctx)
		}
		return err
	}
	return nil
}

// EnsureJobRunning starts job if it isn't currently running.
// If it is already running, this is a no-op.
func EnsureJobRunning(ctx context.Context, job string) error {
	// If the job already has a "start" goal, wait for it to enter the "running" state.
	// This will return nil immediately if it's already start/running, and will return
	// an error immediately if the job has a "stop" goal.
	if err := WaitForJobStatus(ctx, job, StartGoal, RunningState, 0); err == nil {
		return nil
	}

	// Otherwise, start it.
	cmd := testexec.CommandContext(ctx, "initctl", "start", job)
	if err := cmd.Run(); err != nil {
		testing.ContextLogf(ctx, "%q failed: %v", strings.Join(cmd.Args, " "), err)
		cmd.DumpLog(ctx)
		return err
	}
	return nil
}

// WaitForJobStatus waits for job to have the status described by goal/state.
// If job has a goal that doesn't match the requested goal, this function returns an error immediately.
// If timeout is non-zero, it limits the amount of time to wait.
func WaitForJobStatus(ctx context.Context, job string, goal Goal, state State, timeout time.Duration) error {
	// Used to report an out-of-band error if we fail to get the status or see a different goal.
	var statusErr error
	err := testing.Poll(ctx, func(ctx context.Context) error {
		g, s, _, err := JobStatus(ctx, job)
		if err != nil {
			statusErr = err
			return nil
		}
		switch g {
		case goal:
			if s == state {
				return nil
			}
			return fmt.Errorf("status %v/%v", g, s)
		default:
			statusErr = fmt.Errorf("status %v/%v has non-%q goal", g, s, goal)
			return nil
		}
	}, &testing.PollOptions{Timeout: timeout})

	if statusErr != nil {
		return statusErr
	}
	return err
}
