package memoryuser

import (
	"context"
	"fmt"
	"path"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

type AllocationTarget int64

const (
	Host AllocationTarget = iota
	Arc
)

const allocationBinaryPath = "/usr/local/libexec/tast/helpers/local/cros/multivm.Lifecycle.allocate"

type MemoryAllocationManager struct {
	target      AllocationTarget
	allocators  []*MemoryAllocationUnit
	allocateMiB int64
	ratio       float64
	bin         string
	a           *arc.ARC
}

func (m *MemoryAllocationManager) Setup(ctx context.Context) error {
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

func (m *MemoryAllocationManager) AddAllocator(ctx context.Context) error {
	allocatorId := len(m.allocators)
	allocator := NewMemoryAllocationUnit(allocatorId)
	cmd := m.allocationCommand(ctx)
	// Sync... might be slow
	if err := allocator.Run(ctx, cmd); err != nil {
		return errors.Wrapf(err, "failed to run an allocator %d", allocatorId)
	}
	testing.ContextLogf(ctx, "Allocator %d started, pid: %d", allocatorId, allocator.Cmd.Process.Pid)
	m.allocators = append(m.allocators, allocator)
	return nil
}

func (m *MemoryAllocationManager) DeadAllocator() int {
	for _, p := range m.allocators {
		if !p.StillAlive() {
			return p.Id
		}
	}
	return -1
}

func (m *MemoryAllocationManager) TotalAllocatedMiB() int64 {
	return m.allocateMiB * int64(len(m.allocators))
}

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
func NewMemoryAllocationManager(target AllocationTarget, allocateMiB int64, ratio float64, a *arc.ARC) *MemoryAllocationManager {
	return &MemoryAllocationManager{target, []*MemoryAllocationUnit{}, allocateMiB, ratio, "", a}
}

// manager
// - setup execution environment
// - add an allocator and track the allocators
// - record the amount of allocations made so far
// - check alive or not

// unit
// - run
// - stillalive
// - id
// - pid
