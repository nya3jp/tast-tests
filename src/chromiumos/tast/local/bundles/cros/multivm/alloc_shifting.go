// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/multivm/aggregate"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/memory"
	arcMemory "chromiumos/tast/local/memory/arc"
	crostiniMemory "chromiumos/tast/local/memory/crostini"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type shiftingParam struct {
	inHost, inARC, inCrostini bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         AllocShifting,
		Desc:         "Allocates as much memory as possible, then free, in turn across VMs",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "arc_crostini_host",
			Pre:               multivm.ArcCrostiniStarted(),
			Val:               &shiftingParam{inHost: true, inARC: true, inCrostini: true},
			ExtraHardwareDeps: crostini.CrostiniStable,
			ExtraSoftwareDeps: []string{"vm_host", "android_vm"},
			ExtraData:         append(append(crostiniMemory.HelpersData(), crostini.ImageArtifact), arcMemory.HelpersData()...),
		}, {
			Name:              "arc_host",
			Pre:               multivm.ArcStarted(),
			Val:               &shiftingParam{inHost: true, inARC: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData:         arcMemory.HelpersData(),
		}},
	})
}

func AllocShifting(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*shiftingParam)

	var allocLimits []memory.AllocLimit
	if param.inHost {
		alloc, err := memory.NewAnonAlloc(ctx, 1.0, 1000)
		if err != nil {
			s.Fatal("Failed to create host allocator: ", err)
		}
		limit, err := memory.NewAvailableCriticalLimit()
		if err != nil {
			s.Fatal("Failed to create host limit: ", err)
		}
		allocLimits = append(allocLimits, memory.AllocLimit{Alloc: alloc, Limit: limit, Name: "host"})
	}

	if param.inARC {
		if err := arcMemory.PushHelpers(ctx, pre.ARC, s.DataPath); err != nil {
			s.Fatal("Failed to install ARC allocation tools: ", err)
		}
		alloc, err := arcMemory.NewAnonAlloc(ctx, pre.ARC, 1.0, 1000)
		if err != nil {
			s.Fatal("Failed to create ARC allocator: ", err)
		}
		defer func() {
			if err := alloc.Close(ctx); err != nil {
				s.Error("Failed to Close ARC allocation tool: ", err)
			}
		}()
		limit, err := arcMemory.NewPageReclaimLimit(ctx, pre.ARC)
		if err != nil {
			s.Fatal("Failed to create ARC limit: ", err)
		}
		defer func() {
			if err := limit.Close(ctx); err != nil {
				s.Error("Failed to Close ARC limit tool: ", err)
			}
		}()
		allocLimits = append(allocLimits, memory.AllocLimit{Alloc: alloc, Limit: limit, Name: "arc"})
	}

	if param.inCrostini {
		err := crostiniMemory.PushHelpers(ctx, pre.Crostini, s.DataPath)
		defer func() {
			if err := crostiniMemory.Cleanup(ctx, pre.Crostini); err != nil {
				s.Fatal("Failed to clean up Crostini allocation tools: ", err)
			}
		}()
		if err != nil {
			s.Fatal("Failed to install Crostini allocation tools: ", err)
		}
		alloc, err := crostiniMemory.NewAnonAlloc(ctx, pre.Crostini, 1.0, 1000)
		if err != nil {
			s.Fatal("Failed to create Crostini allocator: ", err)
		}
		defer func() {
			if err := alloc.Close(ctx); err != nil {
				s.Error("Failed to Close crostini allocation tool: ", err)
			}
		}()
		limit, err := crostiniMemory.NewPageReclaimLimit(ctx, pre.Crostini)
		if err != nil {
			s.Fatal("Failed to create Crostini limit: ", err)
		}
		defer func() {
			if err := limit.Close(ctx); err != nil {
				s.Error("Failed to Close crostini limit tool: ", err)
			}
		}()
		allocLimits = append(allocLimits, memory.AllocLimit{Alloc: alloc, Limit: limit, Name: "crostini"})
	}

	// Use PageReclaimLimit for the global limit, since it will protect us from
	// OOMing ChromeOS.
	globalLimit, err := memory.NewPageReclaimLimit()
	if err != nil {
		s.Fatal("Failed to create page reclaim limit: ", err)
	}

	if len(allocLimits) == 0 {
		s.Fatal("Test does not specify any allocators")
	}

	minOfMaxAllocated := make([]float64, len(allocLimits))
	for i := 0; i < 3; i++ {
		for j, al := range allocLimits {
			s.Logf("Allocating in %s", al.Name)
			allocated, err := memory.AllocUntilLimit(ctx, 30, globalLimit, al)
			if err != nil {
				s.Fatal("Failed to allocate memory: ", err)
			}
			s.Logf("Freeing in %s", al.Name)
			// TODO: Allocated shoudl return an int64
			if err := al.Alloc.Allocate(ctx, -int64(al.Alloc.Allocated())); err != nil {
				s.Fatal("Failed to free memory: ", err)
			}
			maxAllocated := aggregate.Max(allocated[0]...)
			s.Logf("%s allocated %.0f", al.Name, maxAllocated)
			if i == 0 || maxAllocated < minOfMaxAllocated[j] {
				minOfMaxAllocated[j] = maxAllocated
			}
		}
	}
	p := perf.NewValues()
	for i, minOfMax := range minOfMaxAllocated {
		p.Set(
			perf.Metric{Name: "min_of_max_" + allocLimits[i].Name, Unit: "MiB", Direction: perf.BiggerIsBetter},
			minOfMax,
		)
		s.Logf("%s min of max allocated %.0f MiB", allocLimits[i].Name, minOfMax)
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}
}
