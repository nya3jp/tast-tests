// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"bufio"
	"context"
	"fmt"
	"path"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// CrostiniLifecycleTask launches a process in Crostini which allocates memory.
type CrostiniLifecycleTask struct {
	cont        *vm.Container
	id          int
	allocateMiB int64
	ratio       float64
	limit       memory.Limit
	cmd         *testexec.Cmd
}

// CrostiniLifecycleTask is a MemoryTask.
var _ MemoryTask = (*CrostiniLifecycleTask)(nil)

// CrostiniLifecycleTask is a KillableTask.
var _ KillableTask = (*CrostiniLifecycleTask)(nil)

// Run starts the CrostiniLifecycleTask process, and uses it to allocate memory.
func (t *CrostiniLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	if t.cmd != nil {
		return errors.New("lifecycle already running")
	}

	containerPath, err := containerCrostiniLifecyclePath(ctx, t.cont)
	if err != nil {
		return errors.Wrap(err, "failed to get crostini lifecycle container path")
	}
	t.cmd = t.cont.Command(
		ctx,
		containerPath,
		"0",
		"1000",
		fmt.Sprint(t.allocateMiB),
		fmt.Sprint(t.ratio),
	)

	stdoutPipe, err := t.cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get lifecycle stdout")
	}
	if err := t.cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start Crostini lifecycle")
	}
	// Call Wait so that t.cmd.ProcessState is set as soon as it terminates, so
	// StillAlive can know if it's still alive.
	go func() {
		t.cmd.Wait()
	}()

	// Make sure the output is as expected, and wait until we are done
	// allocating.
	stdout := bufio.NewReader(stdoutPipe)
	if statusString, err := stdout.ReadString('\n'); err != nil {
		return errors.Wrap(err, "failed to read status from Crostini lifecycle")
	} else if !strings.HasPrefix(statusString, "allocating ") {
		return errors.Errorf("failed to read status line, exptected \"allocating ...\", got %q", statusString)
	}
	if doneString, err := stdout.ReadString('\n'); err != nil {
		return errors.Wrap(err, "failed to read done from Crostini lifecycle")
	} else if doneString != "done\n" {
		return errors.Errorf("failed to read done line, exptected \"done\\n\", got %q", doneString)
	}

	return nil
}

// Close kills the CrostiniLifecycleTask process.
func (t *CrostiniLifecycleTask) Close(ctx context.Context, testEnv *TestEnv) {
	if t.cmd == nil {
		return
	}
	t.cmd.Kill()
	t.cmd.Wait(testexec.DumpLogOnError)
}

// String returns a friendly name for the task.
func (t *CrostiniLifecycleTask) String() string {
	return fmt.Sprintf("Crostini Lifecycle %d", t.id)
}

// NeedVM returns false because, while we do need a Crostini VM, we don't want a
// new one created ust for this MemoryTask.
func (t *CrostiniLifecycleTask) NeedVM() bool {
	return false
}

// StillAlive looks in /proc to see if the process is still running.
func (t *CrostiniLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	if t.cmd == nil {
		return false
	}
	// ProcessState is set within the Wait() call in the goroutine started just
	// after we started the lifecycle process.
	return t.cmd.ProcessState == nil
}

// NewCrostiniLifecycleTask foo.
func NewCrostiniLifecycleTask(cont *vm.Container, id int, allocateMiB int64, ratio float64, limit memory.Limit) *CrostiniLifecycleTask {
	var cmd *testexec.Cmd
	return &CrostiniLifecycleTask{cont, id, allocateMiB, ratio, limit, cmd}
}

// NB: this name should be under 15 characters so that killall is able to be
// specific.
const crostiniLifecycleName = "lifecycle"

const crostiniLifecyclePath = "/usr/local/libexec/tast/helpers/local/cros/multivm.Lifecycle.crostini"

func containerCrostiniLifecyclePath(ctx context.Context, cont *vm.Container) (string, error) {
	username, err := cont.GetUsername(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get Crostini user name")
	}
	return path.Join("/home", username, crostiniLifecycleName), nil
}

// InstallCrostiniLifecycle installs the binary needed to run
// CrostiniLifecycleTask.
func InstallCrostiniLifecycle(ctx context.Context, cont *vm.Container) error {
	containerPath, err := containerCrostiniLifecyclePath(ctx, cont)
	if err != nil {
		return errors.Wrap(err, "failed to get crostini lifecycle container path")
	}
	if err := cont.PushFile(ctx, crostiniLifecyclePath, containerPath); err != nil {
		return errors.Wrap(err, "failed to push lifecycle binary to Crostini")
	}
	if err := cont.Command(ctx, "chmod", "755", containerPath).Run(); err != nil {
		return errors.Wrap(err, "failed to make crostini lifecycle binary executable")
	}
	return nil
}

// UninstallCrostiniLifecycle deletes the binary used to run
// CrostiniLifecycleTask, and kills any processes that might still be running.
func UninstallCrostiniLifecycle(ctx context.Context, cont *vm.Container) error {
	if err := cont.Command(ctx, "killall", crostiniLifecycleName).Run(testexec.DumpLogOnError); err != nil {
		// Don't return an error, because killall can fail if no processes were running.
		testing.ContextLog(ctx, "Failed to kill running crostini lifecycle units: ", err)
	}
	containerPath, err := containerCrostiniLifecyclePath(ctx, cont)
	if err != nil {
		return errors.Wrap(err, "failed to get crostini lifecycle container path")
	}
	if err := cont.Command(ctx, "rm", containerPath).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to rm %q", containerPath)
	}
	return nil
}
