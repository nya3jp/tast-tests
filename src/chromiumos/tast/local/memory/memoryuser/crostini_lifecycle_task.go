// Copyright 2021 The ChromiumOS Authors
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

// CrostiniLifecycleUnit launches a process in Crostini which allocates memory.
type CrostiniLifecycleUnit struct {
	cont        *vm.Container
	id          int
	allocateMiB int64
	ratio       float64
	limit       memory.Limit
	cmd         *testexec.Cmd
}

// Run starts the CrostiniLifecycleUnit process, and uses it to allocate memory.
func (t *CrostiniLifecycleUnit) Run(ctx context.Context) error {
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

// Close kills the CrostiniLifecycleUnit process.
func (t *CrostiniLifecycleUnit) Close(ctx context.Context) error {
	if t.cmd == nil {
		return nil
	}
	if t.cmd.ProcessState == nil {
		return nil
	}
	// Only kill the allocator if it is running.
	if err := t.cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill Crostini lifecycle unit")
	}
	if err := t.cmd.Wait(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to wait for Crostini lifecycle unit after kill")
	}
	return nil
}

// StillAlive returns false if the process has exited, or if it never started.
func (t *CrostiniLifecycleUnit) StillAlive(ctx context.Context) bool {
	if t.cmd == nil {
		return false
	}
	// ProcessState is set within the Wait() call in the goroutine started just
	// after we started the lifecycle process.
	return t.cmd.ProcessState == nil
}

// NewCrostiniLifecycleUnit creates a helper to allocate memory inside Crostini.
//
//	id            - A debug ID for logging.
//	allocateBytes - mow much memory to allocate.
//	ratio         - the compression ratio of allocated memory.
//	limit         - if not nil, wait for Limit after allocation.
func NewCrostiniLifecycleUnit(cont *vm.Container, id int, allocateMiB int64, ratio float64, limit memory.Limit) *CrostiniLifecycleUnit {
	var cmd *testexec.Cmd
	return &CrostiniLifecycleUnit{cont, id, allocateMiB, ratio, limit, cmd}
}

// FillCrostiniMemory installs, launches, and allocates in crostini lifecycle
// processes until one is killed, filling up memory in Crostini.
func FillCrostiniMemory(ctx context.Context, cont *vm.Container, unitMiB int64, ratio float64) (func(context.Context) error, error) {
	var units []*CrostiniLifecycleUnit
	cleanup := func(ctx context.Context) error {
		var res error
		for _, unit := range units {
			if err := unit.Close(ctx); err != nil {
				testing.ContextLogf(ctx, "Failed to close CrostiniLifecycleUnit: %s", err)
				if res == nil {
					res = err
				}
			}
		}
		if err := UninstallCrostiniLifecycle(ctx, cont); err != nil {
			testing.ContextLog(ctx, "Failed to clean up CrostiniLifecycleUnit: ", err)
			if res == nil {
				res = err
			}
		}
		return res
	}
	if err := InstallCrostiniLifecycle(ctx, cont); err != nil {
		return cleanup, err
	}
	for i := 0; ; i++ {
		unit := NewCrostiniLifecycleUnit(cont, i, unitMiB, ratio, nil)
		units = append(units, unit)
		if err := unit.Run(ctx); err != nil {
			return cleanup, errors.Wrapf(err, "failed to run CrostiniLifecycleUnit %d", unit.id)
		}
		for _, unit := range units {
			if !unit.StillAlive(ctx) {
				testing.ContextLogf(ctx, "FillChromeOSMemory started %d units of %d MiB before first kill", len(units), unitMiB)
				return cleanup, nil
			}
		}
	}
}

// CrostiniLifecycleTask wraps CrostiniLifecycleTask to conform to the
// MemoryTask and KillableTask interfaces.
type CrostiniLifecycleTask struct{ CrostiniLifecycleUnit }

// CrostiniLifecycleTask is a MemoryTask.
var _ MemoryTask = (*CrostiniLifecycleTask)(nil)

// CrostiniLifecycleTask is a KillableTask.
var _ KillableTask = (*CrostiniLifecycleTask)(nil)

// Run starts the CrostiniLifecycleUnit process, and uses it to allocate memory.
func (t *CrostiniLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	return t.CrostiniLifecycleUnit.Run(ctx)
}

// Close kills the CrostiniLifecycleUnit process.
func (t *CrostiniLifecycleTask) Close(ctx context.Context, testEnv *TestEnv) {
	t.CrostiniLifecycleUnit.Close(ctx)
}

// StillAlive returns false if the process has exited, or if it never started.
func (t *CrostiniLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	return t.CrostiniLifecycleUnit.StillAlive(ctx)
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

// NewCrostiniLifecycleTask creates a helper to allocate memory inside Crostini.
//
//	id            - A debug ID for logging.
//	allocateBytes - mow much memory to allocate.
//	ratio         - the compression ratio of allocated memory.
//	limit         - if not nil, wait for Limit after allocation.
func NewCrostiniLifecycleTask(cont *vm.Container, id int, allocateMiB int64, ratio float64, limit memory.Limit) *CrostiniLifecycleTask {
	return &CrostiniLifecycleTask{*NewCrostiniLifecycleUnit(cont, id, allocateMiB, ratio, limit)}
}

// NB: this name should be under 15 characters so that killall is able to be
// specific.
const crostiniLifecycleName = "lifecycle"

const crostiniLifecyclePath = "/usr/local/libexec/tast/helpers/local/cros/multivm.Lifecycle.allocate"

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
