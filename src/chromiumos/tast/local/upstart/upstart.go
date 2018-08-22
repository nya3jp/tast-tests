// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package upstart supports interacting with the Upstart init daemon on behalf of
// local tests.
package upstart

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

var runningRegexp *regexp.Regexp

func init() {
	runningRegexp = regexp.MustCompile("^[^ ]+ start/running, process (\\d+)$")
}

// JobStatus returns the current status of job.
func JobStatus(ctx context.Context, job string) (running bool, pid int, err error) {
	c := testexec.CommandContext(ctx, "initctl", "status", job)
	b, err := c.Output()
	if err != nil {
		c.DumpLog(ctx)
		return false, 0, err
	}

	out := strings.TrimSpace(string(b))
	if !strings.HasPrefix(out, job+" ") {
		return false, 0, fmt.Errorf("unexpected \"status\" output %q", out)
	}

	m := runningRegexp.FindStringSubmatch(out)
	if m == nil {
		return false, 0, nil
	}

	p, err := strconv.ParseInt(m[1], 10, 32)
	if err != nil {
		return false, 0, fmt.Errorf("unable to parse PID from %q", out)
	}
	return true, int(p), nil
}

// RestartJob restarts job. If the job is currently stopped, it will be started.
func RestartJob(ctx context.Context, job string) error {
	running, _, err := JobStatus(ctx, job)
	if err != nil {
		return err
	}

	var cmd string
	if running {
		cmd = "restart"
	} else {
		cmd = "start"
	}
	c := testexec.CommandContext(ctx, "initctl", cmd, job)
	if err := c.Run(); err != nil {
		c.DumpLog(ctx)
		return fmt.Errorf("restarting job %q failed: %v", job, err)
	}
	return nil
}

// StopJob stops job. If it is not currently running, this is a no-op.
func StopJob(ctx context.Context, job string) error {
	running, _, err := JobStatus(ctx, job)
	if err != nil {
		return err
	}
	if !running {
		return nil
	}
	c := testexec.CommandContext(ctx, "initctl", "stop", job)
	if err := c.Run(); err != nil {
		c.DumpLog(ctx)
		return fmt.Errorf("stopping job %q failed: %v", job, err)
	}
	return nil
}

// EnsureJobRunning starts job if it isn't currently running.
// If it is already running, this is a no-op.
func EnsureJobRunning(ctx context.Context, job string) error {
	running, _, err := JobStatus(ctx, job)
	if err != nil {
		return err
	}
	if !running {
		testing.ContextLogf(ctx, "%v job not running; starting it", job)
		if err = RestartJob(ctx, job); err != nil {
			return err
		}
	}
	return nil
}
