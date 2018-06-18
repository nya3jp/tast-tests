// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testexec

import (
	"context"
	"strings"
	"testing"

	"github.com/shirou/gopsutil/process"
)

func TestKillAll(t *testing.T) {
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

	if status, err := grandchild.Status(); err == nil && status != "Z" {
		t.Errorf("Grandchild process still running: pid=%d, status=%s", grandchild.Pid, status)
	}
}

func TestAutoCollect(t *testing.T) {
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
}
