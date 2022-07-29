// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// MemoryAllocationUnit represents a memory allocation process
// created with multivm.Lifcecycle.allocate.
type MemoryAllocationUnit struct {
	ID    int
	Cmd   *testexec.Cmd
	stdin io.WriteCloser
	ch    (chan error)
}

// Run starts the allocation process.
func (t *MemoryAllocationUnit) Run(ctx context.Context, cmd *testexec.Cmd) error {
	t.Cmd = cmd
	stdoutPipe, err := t.Cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get allocator stdout")
	}
	stdinPipe, err := t.Cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get allocator stdin")
	}
	// We must hold stdin to keep the process running.
	t.stdin = stdinPipe
	// call Run() to immediately notice when the process finished running
	// with StillAlive(). You can synchronously wait for the process by watching ch.
	ch := make(chan error)
	go func() {
		ch <- t.Cmd.Run()
	}()
	t.ch = ch
	// Make sure the output is as expected, and wait until we are done
	// allocating.
	stdout := bufio.NewReader(stdoutPipe)
	if statusString, err := stdout.ReadString('\n'); err != nil {
		return errors.Wrap(err, "failed to read status from the allocation unit")
	} else if !strings.HasPrefix(statusString, "allocating ") {
		return errors.Errorf("failed to read status line, exptected \"allocating ...\", got %q", statusString)
	}
	if doneString, err := stdout.ReadString('\n'); err != nil {
		return errors.Wrap(err, "failed to read done from the Memory allocation unit")
	} else if doneString != "done\n" {
		return errors.Errorf("failed to read done line, exptected \"done\\n\", got %q", doneString)
	}

	return nil
}

// StillAlive checks if the process is still alive.
func (t *MemoryAllocationUnit) StillAlive() bool {
	if t.Cmd == nil {
		return false
	}

	return t.Cmd.ProcessState == nil

}

// Close closes the process by closing the stdin.
// It synchronously checks if the process has really finished
// running by watching ch.
func (t *MemoryAllocationUnit) Close() error {
	if t.Cmd == nil {
		return nil
	}
	t.stdin.Close()
	// t.ch returns an error because it exits with a non-zero status.
	// It is intended, so we just ignore the error.
	select {
	case <-t.ch:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("failed to close an allocator process")
	}
}

// NewMemoryAllocationUnit initializes a MemoryAllocationUnit.
func NewMemoryAllocationUnit(id int) *MemoryAllocationUnit {
	var cmd *testexec.Cmd
	return &MemoryAllocationUnit{id, cmd, nil, nil}
}
