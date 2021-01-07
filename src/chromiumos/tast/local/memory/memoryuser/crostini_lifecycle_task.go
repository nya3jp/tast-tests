// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"path"
	"runtime"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
)

// CrostiniLifecycleTask launches a process in Crostini which allocates memory.
type CrostiniLifecycleTask struct {
	id          int
	allocateMiB int64
	cont        *vm.Container
}

// CrostiniLifecycleTask is a MemoryTask.
var _ MemoryTask = (*CrostiniLifecycleTask)(nil)

// Run starts the CrostiniLifecycleTask process, and uses it to allocate memory.
func (t *CrostiniLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	cmd := t.cont.Command(ctx, "/home/foo/alloc", "1000", "0", fmt.Sprint(t.allocateMiB), "0.67")
	cmd.Start()

	// maybe it's better to run a script that execs the tool and returns the pid?
	// then we can kill it later easily
	// but how to wait for it to finish allocating?

	return nil
}

// Close kills the CrostiniLifecycleTask process.
func (t *CrostiniLifecycleTask) Close(ctx context.Context, testEnv *TestEnv) {

}

// String returns a friendly st
func (t *CrostiniLifecycleTask) String() string {
	return fmt.Sprintf("Crostini Lifecycle %d", t.id)
}

// NeedVM returns false because, while we do need a Crostini VM, we don't want a
// new one created ust for this MemoryTask.
func (t *CrostiniLifecycleTask) NeedVM() bool {
	return false
}

var crostiniDataByArch = map[string]string{"amd64": "crostini_lifecycle_amd64"}

// CrostiniLifecycleData returns the data dependencies for a test that will call
// InstallCrostiniLifecycle.
func CrostiniLifecycleData() []string {
	if data, ok := crostiniDataByArch[runtime.GOARCH]; ok {
		return []string{data}
	}
	// Return an empty slice, InstallCrostiniLifecycle will raise an error.
	return []string{}
}

// InstallCrostiniLifecycle installs the binary needed to run
// CrostiniLifecycleTask.
func InstallCrostiniLifecycle(ctx context.Context, cont *vm.Container, dataPath func(string) string) error {
	data, ok := crostiniDataByArch[runtime.GOARCH]
	if !ok {
		return errors.Errorf("no Crostini Lifecycle executable for %s", runtime.GOARCH)
	}

	user, err := cont.GetUsername(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get Crostini user name")
	}
	if err := cont.PushFile(ctx, dataPath(data), path.Join("/home", user, "lifecycle")); err != nil {
		return errors.Wrap(err, "failed to push alloc tool to Crostini")
	}

	//cont.PushFile(ctx, dataPath("alloc-x86_64"), "/home/foo/alloc")
	return nil
}
