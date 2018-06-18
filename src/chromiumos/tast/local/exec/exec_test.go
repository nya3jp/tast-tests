// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/shirou/gopsutil/process"
)

func TestKillAll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	if !strings.Contains(cmd.Log.String(), "foo") {
		t.Error("Run: stdout should be collected")
	}
	if !strings.Contains(cmd.Log.String(), "bar") {
		t.Error("Run: stderr should be collected")
	}

	cmd = CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2")
	if _, err := cmd.Output(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd.Log.String(), "foo") {
		t.Error("Output: stdout should NOT be collected")
	}
	if !strings.Contains(cmd.Log.String(), "bar") {
		t.Error("Output: stderr should be collected")
	}

	cmd = CommandContext(context.Background(), "sh", "-c", "echo foo; echo bar >&2")
	if _, err := cmd.OutputCombined(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd.Log.String(), "foo") {
		t.Error("OutputCombined: stdout should NOT be collected")
	}
	if strings.Contains(cmd.Log.String(), "bar") {
		t.Error("OutputCombined: stderr should NOT be collected")
	}
}
