// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"
	"fmt"
	"path"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
)

// AllocationTarget is an enum listing up the available target of
// allocation processes in MemoryAllocationCanaryHealthPerf.
type AllocationTarget int

const (
	// Host means that the manager allocates memory in the host ChromeOS.
	Host AllocationTarget = iota
	// Arc means that the manager allocates memory in the guest ARCVM.
	Arc
)

const allocationBinaryPath = "/usr/local/libexec/tast/helpers/local/cros/multivm.Lifecycle.allocate"

// MemoryAllocationManager creates MemoryAllocationUnits depending on
// the specified AllocationTarget.
type MemoryAllocationManager struct {
	target      AllocationTarget
	allocators  []*MemoryAllocationUnit
	allocateMiB int64
	ratio       float64
	bin         string
	a           *arc.ARC
}

// setup does setup for specified target and set the place of the binary
// to bin.
func (m *MemoryAllocationManager) setup(ctx context.Context) error {
	switch m.target {
	case Host:
		m.bin = allocationBinaryPath
	case Arc:
		tempDir, err := m.a.TempDir(ctx)
		if err != nil {
			return errors.Wrap(err, "cannot create a temp dir in ARC")
		}
		if err := m.a.PushFile(ctx, allocationBinaryPath, tempDir); err != nil {
			return errors.Wrap(err, "failed to push the ARC allocation binary")
		}
		m.bin = path.Join(tempDir, path.Base(allocationBinaryPath))
		if err := m.a.Command(ctx, "chmod", "0755", m.bin).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to give permission the ARC allocation binary")
		}
	}
	return nil
}

// allocationCommand creates a command executes multivm.Lifecycle.allocate.
func (m *MemoryAllocationManager) allocationCommand(ctx context.Context) *testexec.Cmd {
	switch m.target {
	case Host:
		return testexec.CommandContext(ctx, m.bin, "0", "1000", fmt.Sprint(m.allocateMiB), fmt.Sprint(m.ratio))
	case Arc:
		return m.a.Command(ctx, m.bin, "0", "1000", fmt.Sprint(m.allocateMiB), fmt.Sprint(m.ratio))
	}
	// Never reaches
	return nil
}

// AddAllocator creates a new allocator under the MemoryAllocationManager.
func (m *MemoryAllocationManager) AddAllocator(ctx context.Context) error {
	allocatorID := len(m.allocators)
	allocator := NewMemoryAllocationUnit(allocatorID)
	cmd := m.allocationCommand(ctx)
	// Sync... might be slow
	if err := allocator.Run(ctx, cmd); err != nil {
		return errors.Wrapf(err, "failed to run an allocator %d", allocatorID)
	}
	m.allocators = append(m.allocators, allocator)
	return nil
}

// AssertNoDeadAllocator returns an error containing the id of the dead allocator
// if there is any.
func (m *MemoryAllocationManager) AssertNoDeadAllocator() error {
	for _, p := range m.allocators {
		if !p.StillAlive() {
			return errors.Errorf("Allocator %d is dead", p.ID)
		}
	}
	return nil
}

// TotalAllocatedMiB returns the amount of memory allocated by the allocators
// in MiB.
func (m *MemoryAllocationManager) TotalAllocatedMiB() int64 {
	return m.allocateMiB * int64(m.NumOfAllocators())
}

// NumOfAllocators returns the number of the alloctors the MemoryAllocationManager has.
func (m *MemoryAllocationManager) NumOfAllocators() int {
	return len(m.allocators)
}

// Cleanup kills all the allocators. It also removes the temp directories if needed.
func (m *MemoryAllocationManager) Cleanup(ctx context.Context) error {
	for _, u := range m.allocators {
		if err := u.Close(); err != nil {
			return errors.Wrap(err, "failed to kill allocator process")
		}
	}
	if m.target == Arc {
		tempDir := path.Dir(m.bin)
		if err := m.a.RemoveAll(ctx, tempDir); err != nil {
			return errors.Wrap(err, "failed to remove the temp dir")
		}
	}
	return nil
}

// NewMemoryAllocationManager creates a MemoryAllocationManager for AllocationTarget.
func NewMemoryAllocationManager(ctx context.Context, target AllocationTarget, allocateMiB int64, ratio float64, a *arc.ARC) *MemoryAllocationManager {
	mgr := &MemoryAllocationManager{target, []*MemoryAllocationUnit{}, allocateMiB, ratio, "", a}
	mgr.setup(ctx)
	return mgr
}
