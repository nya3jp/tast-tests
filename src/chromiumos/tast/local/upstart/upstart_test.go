// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package upstart

import (
	"fmt"
	"testing"
)

func TestParseStatus(t *testing.T) {
	for _, tc := range []struct {
		job, line string
		goal      Goal
		state     State
		pid       int
	}{
		{"powerd", "powerd start/running, process 9398\n", StartGoal, RunningState, 9398},
		{"boot-splash", "boot-splash stop/waiting\n", StopGoal, WaitingState, 0},
		{"ureadahead", "ureadahead stop/pre-stop, process 227\npre-stop process 5579\n", StopGoal, PreStopState, 227},
	} {
		goal, state, pid, err := parseStatus(tc.job, tc.line)
		sig := fmt.Sprintf("parseStatus(%q, %q)", tc.job, tc.line)
		if err != nil {
			t.Errorf("%s returned error: %v", sig, err)
			continue
		}
		if goal != tc.goal {
			t.Errorf("%s returned goal %q; want %q", sig, goal, tc.goal)
		}
		if state != tc.state {
			t.Errorf("%s returned state %q; want %q", sig, state, tc.state)
		}
		if pid != tc.pid {
			t.Errorf("%s returned PID %d; want %d", sig, pid, tc.pid)
		}
	}
}
