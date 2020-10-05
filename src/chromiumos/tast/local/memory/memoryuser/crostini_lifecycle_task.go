// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// CrostiniLifecycleTask launches a process in Crostini that allocates memory.
type CrostiniLifecycleTask struct {
	id          int
	cont        *vm.Container
	bytes       int64
	ratio       float64
	oomScoreAdj int64
	limit       memory.Limit
	alloc       memory.Alloc
}

// ArcLifecycleTask is a MemoryTest.
var _ MemoryTask = (*CrostiniLifecycleTask)(nil)

// Run launches a process in Crostini and allocates memory.
func (t *CrostiniLifecycleTask) Run(ctx context.Context, testEnv *TestEnv) error {
	if t.alloc != nil {
		return errors.New("alloc already exists (maybe Run was called more than once?)")
	}
	alloc, err := memory.NewCrostiniAnonAlloc(ctx, t.cont, t.ratio, t.oomScoreAdj)
	if err != nil {
		return errors.Wrap(err, "failed to make Alloc for CrostiniLifecycleTask")
	}
	t.alloc = alloc
	if err := t.alloc.Allocate(ctx, t.bytes); err != nil {
		return errors.Wrap(err, "failed to allocate for CrostiniLifecycleTask")
	}
	return nil
}

// Close stops the Crostini process.
func (t *CrostiniLifecycleTask) Close(ctx context.Context, testEnv *TestEnv) {
	if t.alloc == nil {
		return
	}
	if err := t.alloc.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to Close CrostiniLifecycleTask Alloc: ", err)
	}
}

// String lets callers know this is a Crostini lifecycle task.
func (t *CrostiniLifecycleTask) String() string {
	return fmt.Sprintf("Crostini Lifecycle Task #%d", t.id)
}

// NeedVM returns false because we do not want to launch a new VM.
func (t *CrostiniLifecycleTask) NeedVM() bool {
	return false
}

// StillAlive checks to see if the process is still responding to allocation
// requests.
func (t *CrostiniLifecycleTask) StillAlive(ctx context.Context, testEnv *TestEnv) bool {
	if t.alloc == nil {
		return false
	}
	return nil == t.alloc.Allocate(ctx, 0)
}

// NewCrostiniLifecycleTask creates a MemoryTask that allocates memory on
//Crostini.
func NewCrostiniLifecycleTask(cont *vm.Container, id int, bytes int64, ratio float64, oomScoreAdj int64, limit memory.Limit) *CrostiniLifecycleTask {
	return &CrostiniLifecycleTask{
		id:          id,
		cont:        cont,
		bytes:       bytes,
		ratio:       ratio,
		oomScoreAdj: oomScoreAdj,
		limit:       limit,
		alloc:       nil,
	}
}
