// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package upstart interacts with the Upstart init daemon on behalf of local tests.
package upstart

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

const (
	statefulPartitionDir = "/mnt/stateful_partition"
	upstartJobDir        = "/etc/init"
	uiJob                = "ui" // special-cased in StopJob due to its respawning behavior
)

var statusRegexp *regexp.Regexp
var initctlUnknownRegexp *regexp.Regexp
var allGoals map[upstart.Goal]struct{}
var allStates map[upstart.State]struct{}

func init() {
	// Matches a leading line of e.g. "ui start/running, process 3182" or "boot-splash stop/waiting".
	// Also supports job instance, e.g. "ml-service (mojo_service) start/running, process 712".

	statusRegexp = regexp.MustCompile(`(?m)^[^ ]+ (?:\([^ ]+\) )?([-a-z]+)/([-a-z]+)(?:, process (\d+))?$`)
	initctlUnknownRegexp = regexp.MustCompile(`(?m)^initctl: Unknown ([a-z]+): (.*)?$`)

	allGoals = map[upstart.Goal]struct{}{upstart.StartGoal: {}, upstart.StopGoal: {}}

	allStates = make(map[upstart.State]struct{})
	for _, s := range []upstart.State{
		upstart.WaitingState,
		upstart.StartingState,
		upstart.SecurityState,
		upstart.PreStartState,
		upstart.SpawnedState,
		upstart.PostStartState,
		upstart.RunningState,
		upstart.PreStopState,
		upstart.StoppingState,
		upstart.KilledState,
		upstart.PostStopState} {
		allStates[s] = struct{}{}
	}
}

// JobStatus returns the current status of job.
// If the PID is unavailable (i.e. the process is not running), 0 will be returned.
// An error will be returned if the job is unknown (i.e. it has no config in /etc/init).
// args is passed to the job as extra parameters, e.g. KEY=value for multiple-instance jobs.
func JobStatus(ctx context.Context, job string, args ...string) (goal upstart.Goal, state upstart.State, pid int, err error) {
	stdout, stderr, err := testexec.CommandContext(ctx, "initctl", append([]string{"status", job}, args...)...).SeparatedOutput(testexec.DumpLogOnError)

	// Tolerates err if stderr starts with "initctl: Unknown instance". It happens when
	// job is multiple-instance, and the specific instance is treated as stop/waiting.
	if err != nil {
		if strings.HasPrefix(string(stderr), "initctl: Unknown instance") {
			return upstart.StopGoal, upstart.WaitingState, pid, nil
		}
		return goal, state, pid, err
	}

	return parseStatus(job, string(stdout))
}

// parseStatus parses the output from "initctl status <job>", e.g. "ui start/running, process 28515".
// Also supports job instance status. e.g. "ml-service (mojo_service) start/running, process 6820".
// The output may be multiple lines; see the example in Section 10.1.6.19.3,
// "Single Job Instance Running with Multiple PIDs", in the Upstart Cookbook.
func parseStatus(job, out string) (goal upstart.Goal, state upstart.State, pid int, err error) {
	if !strings.HasPrefix(out, job+" ") {
		return goal, state, pid, errors.Errorf("missing job prefix %q in %q", job, out)
	}
	m := statusRegexp.FindStringSubmatch(out)
	if m == nil {
		return goal, state, pid, errors.Errorf("unexpected format in %q", out)
	}

	goal = upstart.Goal(m[1])
	if _, ok := allGoals[goal]; !ok {
		return goal, state, pid, errors.Errorf("invalid goal %q", m[1])
	}

	state = upstart.State(m[2])
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

// CheckJob checks the named upstart job (and the specified instance) and
// returns an error if it isn't running or has a process in the zombie state.
func CheckJob(ctx context.Context, job string, args ...string) error {
	if goal, state, pid, err := JobStatus(ctx, job, args...); err != nil {
		return errors.Wrapf(err, "failed to get %v status", job)
	} else if goal != upstart.StartGoal || state != upstart.RunningState {
		return errors.Errorf("%v not running (%v/%v)", job, goal, state)
	} else if proc, err := process.NewProcess(int32(pid)); err != nil {
		return errors.Wrapf(err, "failed to check %v process %d", job, pid)
	} else if status, err := proc.Status(); err != nil {
		return errors.Wrapf(err, "failed to get %v process %d status", job, pid)
	} else if status == "Z" {
		return errors.Errorf("%v process %d is a zombie", job, pid)
	}
	return nil
}

// JobExists returns true if the supplied job exists (i.e. it has a config file known by Upstart).
// err can be non-nil if job is multiple-instance, so we need also to check stderr
// If stderr starts with "initctl: Unknown job:", it means this job doesn't exist.
func JobExists(ctx context.Context, job string) bool {
	_, stderr, err := testexec.CommandContext(ctx, "initctl", "status", job).SeparatedOutput(testexec.DumpLogOnError)

	if err != nil && strings.HasPrefix(string(stderr), "initctl: Unknown job") {
		return false
	}

	return true
}

// RestartJob restarts the job (single-instance) or the specified instance of
// the job (multiple-instance). If the job (instance) is currently stopped, it will be started.
// Note that the job is reloaded if it is already running; this differs from the
// "initctl restart" behavior as described in Section 10.1.2, "restart", in the Upstart Cookbook.
// args is passed to the job as extra parameters.
func RestartJob(ctx context.Context, job string, args ...string) error {
	// Make sure that the job isn't running and then try to start it.
	if err := StopJob(ctx, job, args...); err != nil {
		return errors.Wrapf(err, "stopping %q failed", append([]string{job}, args...))
	}
	if err := StartJob(ctx, job, args...); err != nil {
		return errors.Wrapf(err, "starting %q failed", append([]string{job}, args...))
	}
	return nil
}

// StopJob stops job or the specified job instance. If it is not currently running, this is a no-op.
//
// The ui job receives special behavior since it is restarted out-of-band by the ui-respawn
// job when session_manager exits. To work around this, when job is "ui", this function first
// waits for the job to reach a stable state. See https://crbug.com/891594.
func StopJob(ctx context.Context, job string, args ...string) error {
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
	testexec.CommandContext(ctx, "initctl", append([]string{"stop", job}, args...)...).Run(testexec.DumpLogOnError)

	// If the job was already stopped, the above "initctl stop" would have failed.
	// Check its actual status now.
	return WaitForJobStatus(ctx, job, upstart.StopGoal, upstart.WaitingState, RejectWrongGoal, 0, args...)
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
		uiStable := ug == upstart.StartGoal || (ug == upstart.StopGoal && us == upstart.WaitingState)

		rg, rs, _, _ := JobStatus(ctx, respawnJob)
		respawnStopped := rg == upstart.StopGoal && rs == upstart.WaitingState

		if !uiStable || !respawnStopped {
			return errors.Errorf("%v status %v/%v, %v status %v/%v", uiJob, ug, us, respawnJob, rg, rs)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// DumpJobs writes the snapshot of all jobs' status to path.
func DumpJobs(ctx context.Context, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd := testexec.CommandContext(ctx, "initctl", "list")
	cmd.Stdout = f
	return cmd.Run(testexec.DumpLogOnError)
}

// EnsureJobRunning starts job if it isn't currently running.
// If it is already running, this is a no-op.
func EnsureJobRunning(ctx context.Context, job string, args ...string) error {
	// If the job already has a "start" goal, wait for it to enter the "running" state.
	// This will return nil immediately if it's already start/running, and will return
	// an error immediately if the job has a "stop" goal.
	if err := WaitForJobStatus(ctx, job, upstart.StartGoal, upstart.RunningState, RejectWrongGoal, 0, args...); err == nil {
		return nil
	}

	// Otherwise, start it. This command blocks until the job enters the "running" state.
	return StartJob(ctx, job, args...)
}

// StartJob starts job. If it is already running, this returns an error.
// args is passed to the job as extra parameters.
func StartJob(ctx context.Context, job string, args ...string) error {
	return testexec.CommandContext(ctx, "initctl", append([]string{"start", job}, args...)...).Run(testexec.DumpLogOnError)
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
func WaitForJobStatus(ctx context.Context, job string, goal upstart.Goal, state upstart.State,
	gp GoalPolicy, timeout time.Duration, args ...string) error {
	// Used to report an out-of-band error if we fail to get the status or see a different goal.
	return testing.Poll(ctx, func(ctx context.Context) error {
		g, s, _, err := JobStatus(ctx, job, args...)
		if err != nil {
			return testing.PollBreak(err)
		}
		if g != goal && gp == RejectWrongGoal {
			return testing.PollBreak(errors.Errorf("status %v/%v has non-%q goal", g, s, goal))
		}
		if g != goal || s != state {
			return errors.Errorf("status %v/%v", g, s)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

// DisableJob disables the given upstart job, which takes effect on the next
// reboot. The rootfs must be writable when this function is called.
func DisableJob(job string) error {
	jobFile := job + ".conf"
	currentJobPath := filepath.Join(upstartJobDir, jobFile)
	newJobPath := filepath.Join(statefulPartitionDir, jobFile)
	return fsutil.MoveFile(currentJobPath, newJobPath)
}

// IsJobEnabled returns true if the given upstart job is enabled.
func IsJobEnabled(job string) (bool, error) {
	jobFile := job + ".conf"
	jobPath := filepath.Join(upstartJobDir, jobFile)
	if _, err := os.Stat(jobPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// EnableJob enables the given upstart job. The job must have already been
// disabled with DisableJob before this function is called. The rootfs must be
// writable when this function is called.
func EnableJob(job string) error {
	jobFile := job + ".conf"
	currentJobPath := filepath.Join(statefulPartitionDir, jobFile)
	newJobPath := filepath.Join(upstartJobDir, jobFile)
	return fsutil.MoveFile(currentJobPath, newJobPath)
}
