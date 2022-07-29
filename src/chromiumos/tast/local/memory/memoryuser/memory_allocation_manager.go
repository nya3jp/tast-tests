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

type MemoryAllocationManager interface {
	Setup(ctx context.Context) error
	AddAllocator(ctx context.Context) error
	DeadAllocator() int
	TotalAllocatedMiB() int64
	Cleanup(ctx context.Context) error
}

type HostMemoryAllocationManager struct {
	allocators  []*MemoryAllocationUnit
	allocateMiB int64
	ratio       float64
	bin         string
}

func (m *HostMemoryAllocationManager) Setup(ctx context.Context) error {
	m.bin = allocationBinaryPath
	return nil
}

func (m *HostMemoryAllocationManager) AddAllocator(ctx context.Context) error {
	allocatorId := len(m.allocators)
	allocator := NewMemoryAllocationUnit(allocatorId)
	allocationCommand := testexec.CommandContext(ctx, m.bin, "0", "1000", fmt.Sprint(m.allocateMiB), fmt.Sprint(m.ratio))
	// Sync... might be slow
	if err := allocator.Run(ctx, allocationCommand); err != nil {
		return errors.Wrapf(err, "failed to run an allocator %d", allocatorId)
	}
	testing.ContextLogf(ctx, "Allocator %d started, pid: %d", allocatorId, allocator.Cmd.Process.Pid)
	m.allocators = append(m.allocators, allocator)
	return nil
}

func (m *HostMemoryAllocationManager) DeadAllocator() int {
	for _, p := range m.allocators {
		if !p.StillAlive() {
			return p.Id
		}
	}
	return -1
}

func (m *HostMemoryAllocationManager) TotalAllocatedMiB() int64 {
	return m.allocateMiB * int64(len(m.allocators))
}

func (m *HostMemoryAllocationManager) Cleanup(ctx context.Context) error {
	for _, u := range m.allocators {
		if err := u.Close(); err != nil {
			return errors.Wrap(err, "failed to kill allocator process")

		}
	}

	return nil
}

type ArcMemoryAllocationManager struct {
	allocators  []*MemoryAllocationUnit
	a           *arc.ARC
	allocateMiB int64
	ratio       float64
	bin         string
}

func (m *ArcMemoryAllocationManager) Setup(ctx context.Context) error {
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
	return nil
}

func (m *ArcMemoryAllocationManager) Cleanup(ctx context.Context) error {
	for _, u := range m.allocators {
		if err := u.Close(); err != nil {
			return errors.Wrap(err, "failed to kill allocator process")

		}
	}

	tempDir := path.Dir(m.bin)
	if err := m.a.RemoveAll(ctx, tempDir); err != nil {
		return errors.Wrap(err, "failed to remove the temp dir")

	}
	return nil
}

func (m *ArcMemoryAllocationManager) AddAllocator(ctx context.Context) error {
	allocatorId := len(m.allocators)
	allocator := NewMemoryAllocationUnit(allocatorId)
	allocationCommand := m.a.Command(ctx, m.bin, "0", "1000", fmt.Sprint(m.allocateMiB), fmt.Sprint(m.ratio))
	// Sync... might be slow
	if err := allocator.Run(ctx, allocationCommand); err != nil {
		return errors.Wrapf(err, "failed to run an allocator %d", allocatorId)
	}
	testing.ContextLogf(ctx, "Allocator %d started, pid: %d", allocatorId, allocator.Cmd.Process.Pid)
	m.allocators = append(m.allocators, allocator)
	return nil
}

func (m *ArcMemoryAllocationManager) DeadAllocator() int {
	for _, p := range m.allocators {
		if !p.StillAlive() {
			return p.Id
		}
	}
	return -1
}

func (m *ArcMemoryAllocationManager) TotalAllocatedMiB() int64 {
	return m.allocateMiB * int64(len(m.allocators))
}

func NewMemoryAllocationManager(target AllocationTarget, allocateMiB int64, ratio float64, a *arc.ARC) MemoryAllocationManager {
	switch target {
	case Host:
		return &HostMemoryAllocationManager{[]*MemoryAllocationUnit{}, allocateMiB, ratio, ""}
	case Arc:
		return &ArcMemoryAllocationManager{[]*MemoryAllocationUnit{}, a, allocateMiB, ratio, ""}
	}
	// Never reaches
	return nil
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
