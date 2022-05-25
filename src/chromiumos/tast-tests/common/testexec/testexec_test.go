// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testexec

import (
	"context"
	"os/exec"
	"strings"
	gotesting "testing"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	tastexec "chromiumos/tast/exec"
	"chromiumos/tast/testing"
)

func TestKillAll(t *gotesting.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if cancel != nil {
			cancel()
		}
	}()

	cmd := CommandContext(ctx, "sh", "-c", "sleep 60; true")
	if err := cmd.Start(); err != nil {
		t.Fatal("Failed to start a shell: ", err)
	}

	var grandchild *process.Process
	for grandchild == nil {
		ps, err := process.Processes()
		if err != nil {
			t.Fatal("Failed to enumerate processes: ", err)
		}

		for _, p := range ps {
			ppid, err := p.Ppid()
			if err == nil && int(ppid) == cmd.Process.Pid {
				grandchild = p
				break
			}
		}
	}

	cancel()
	cancel = nil

	cmd.Wait()

	// cancel() above should trigger sending SIGKILL to both the child and the grandchild processes.
	// However, there's no guarantee of the order, so there is small chance that the grandchild
	// is still alive at this moment.
	// Thus, poll until the grandchild is gone for a short period. In most cases it should
	// be done immediately, so wait for 3 secs (arbitrary time).
	if err := testing.Poll(context.Background(), func(ctx context.Context) error {
		if status, err := grandchild.Status(); err == nil {
			return errors.Errorf("grandchild process still running: pid=%d, status=%s", grandchild.Pid, status)
		}
		// The grandchild process is terminated and collected.
		return nil
	}, &testing.PollOptions{Timeout: 3 * time.Second}); err != nil {
		t.Error("Grandchild process was not killed on cancel: ", err)
	}
}

func TestAutoCollect(t *gotesting.T) {
	cmd := CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd.log.String(), "foo") {
		t.Errorf("Run: log %q does not contain %q", cmd.log.String(), "foo")
	}
	if !strings.Contains(cmd.log.String(), "bar") {
		t.Errorf("Run: log %q does not contain %q", cmd.log.String(), "bar")
	}

	cmd = CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2")
	if _, err := cmd.Output(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd.log.String(), "foo") {
		t.Errorf("Output: log %q contains %q", cmd.log.String(), "foo")
	}
	if !strings.Contains(cmd.log.String(), "bar") {
		t.Errorf("Output: log %q does not contain %q", cmd.log.String(), "bar")
	}

	cmd = CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2")
	if _, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd.log.String(), "foo") {
		t.Errorf("CombinedOutput: log %q contains %q", cmd.log.String(), "foo")
	}
	if strings.Contains(cmd.log.String(), "bar") {
		t.Errorf("CombinedOutput: log %q contains %q", cmd.log.String(), "bar")
	}

	cmd = CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2")
	if _, _, err := cmd.SeparatedOutput(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd.log.String(), "foo") {
		t.Errorf("SeparatedOutput: log %q contains %q", cmd.log.String(), "foo")
	}
	if strings.Contains(cmd.log.String(), "bar") {
		t.Errorf("SeparatedOutput: log %q contains %q", cmd.log.String(), "bar")
	}
}

func TestOutput(t *gotesting.T) {
	b, err := CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2").Output()
	if err != nil {
		t.Error(err)
	} else {
		s := string(b)
		if !strings.Contains(s, "foo") {
			t.Errorf("Output: %q does not contain %q", s, "foo")
		}
		if strings.Contains(s, "bar") {
			t.Errorf("Output: %q contains %q", s, "bar")
		}
	}

	b, err = CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2").CombinedOutput()
	if err != nil {
		t.Error(err)
	} else {
		s := string(b)
		if !strings.Contains(s, "foo") {
			t.Errorf("CombinedOutput: %q does not contain %q", s, "foo")
		}
		if !strings.Contains(s, "bar") {
			t.Errorf("CombinedOutput: %q does not contain %q", s, "bar")
		}
	}

	bo, be, err := CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2").SeparatedOutput()
	if err != nil {
		t.Error(err)
	} else {
		so := string(bo)
		if !strings.Contains(so, "foo") {
			t.Errorf("SeparatedOutput: stdout %q does not contain %q", so, "foo")
		}
		if strings.Contains(so, "bar") {
			t.Errorf("SeparatedOutput: stdout %q contains %q", so, "bar")
		}

		se := string(be)
		if strings.Contains(se, "foo") {
			t.Errorf("SeparatedOutput: stderr %q contains %q", se, "foo")
		}
		if !strings.Contains(se, "bar") {
			t.Errorf("SeparatedOutput: stderr %q does not contain %q", se, "bar")
		}
	}
}

func TestGetWaitStatus(t *gotesting.T) {
	err28 := exec.Command("sh", "-c", "exit 28").Run()

	for _, c := range []struct {
		err  error
		code int
		ok   bool
	}{
		{nil, 0, true},
		{err28, 28, true},
		{errors.New("foo"), 0, false},
	} {
		status, ok := GetWaitStatus(c.err)
		code := status.ExitStatus()
		if ok != c.ok || status.ExitStatus() != c.code {
			t.Errorf("GetWaitStatus(%#v) = (%v, %v); want (%v, %v)", c.err, code, ok, c.code, c.ok)
		}
	}
}

func TestExitCode(t *gotesting.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err28 := exec.Command("sh", "-c", "exit 28").Run()
	killed := CommandContext(ctx, "sleep", "3s")
	if err := killed.Start(); err != nil {
		t.Fatal("Failed to start a process: ", err)
	}
	if err := killed.Kill(); err != nil {
		t.Fatal("Failed to kill a process: ", err)
	}
	errKilled := killed.Wait()

	for _, c := range []struct {
		err  error
		code int
		ok   bool
	}{
		{nil, 0, true},
		{err28, 28, true},
		{errKilled, 128 + 9 /* SIGKILL */, true},
		{errors.New("foo"), 0, false},
	} {
		code, ok := ExitCode(c.err)
		if ok != c.ok || code != c.code {
			t.Errorf("ExitCode(%#v) = (%v, %v); want (%v, %v)", c.err, code, ok, c.code, c.ok)
		}
	}
}

type sshOrLocal interface {
	Run(opts ...tastexec.RunOption) error
	Output(opts ...tastexec.RunOption) ([]byte, error)
	CombinedOutput(opts ...tastexec.RunOption) ([]byte, error)
	Start() error
	Wait(opts ...tastexec.RunOption) error
	DumpLog(ctx context.Context) error
}

// TestCast verifies the return value of CommandContext can be assigned to an interface that also works for ssh Cmd.
func TestCast(t *gotesting.T) {
	var cmd sshOrLocal
	cmd = CommandContext(context.Background(), "true")
	cmd.Run()
}
