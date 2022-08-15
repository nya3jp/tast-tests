// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type canaryHealthPerfParam struct {
	canary           memoryuser.CanaryType
	allocationTarget memoryuser.AllocationTarget
}

const iterationsVar = "multivm.MemoryAllocationCanaryHealthPerf.iterations"

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryAllocationCanaryHealthPerf,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "How much memory can we allocate before the specified canary dies",
		Contacts: []string{
			"kokiryu@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "tab_host",
			Pre:  multivm.NoVMStarted(),
			Val:  &canaryHealthPerfParam{memoryuser.Tab, memoryuser.Host},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_host",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.App, memoryuser.Host},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.App, memoryuser.Arc},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Vars: []string{
			iterationsVar,
		},
		Timeout: 30 * time.Minute,
	})
}

const increasedLimit = 4096

// updateLimit updates the limit of the number of open files by the test process.
// We need this because this test opens one file (stdin) for one memory allocator.
func updateLimit(ctx context.Context) error {
	pid := os.Getpid()
	cmd := testexec.CommandContext(ctx, "prlimit", fmt.Sprintf("--pid=%d", pid), fmt.Sprintf("--nofile=%d:%d", increasedLimit, increasedLimit))
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to increase the open file limit")
	}
	return nil
}

const canaryAllocatedMiB = 25
const canaryCompressionRatio = 0.67
const singleAllocatorMiB = 50
const allocatorComplessionRatio = 0.67

func stressCanary(ctx context.Context, s *testing.State, param *canaryHealthPerfParam, cr *chrome.Chrome, a *arc.ARC) (int64, time.Duration, error) {
	canary, err := memoryuser.NewCanary(ctx, param.canary, canaryAllocatedMiB, canaryCompressionRatio, s.DataFileSystem(), cr, a)
	if err != nil {
		return -1, -1, errors.Wrap(err, "failed to create the canary")
	}
	canary.Run(ctx)
	defer canary.Close(ctx)

	target := param.allocationTarget
	allocationManager := memoryuser.NewMemoryAllocationManager(target, singleAllocatorMiB, allocatorComplessionRatio, a)
	allocationManager.Setup(ctx)
	defer allocationManager.Cleanup(ctx)

	var allocationTime time.Duration = 0
	var allocatedMiB int64 = 0

	for true {
		if !canary.StillAlive(ctx) {
			s.Logf("%s died after %d MiB allocations", canary.String(), allocationManager.TotalAllocatedMiB())
			allocatedMiB += allocationManager.TotalAllocatedMiB()
			break
		}
		if id := allocationManager.DeadAllocator(); id >= 0 {
			return -1, -1, errors.Errorf("allocator %d is killed before the canary", id)
		}
		start := time.Now()
		if err := allocationManager.AddAllocator(ctx); err != nil {
			return -1, -1, errors.Wrap(err, "failed to add an allocator")
		}
		elapsed := time.Since(start)
		s.Log("Added an allocator")
		allocationTime += elapsed
	}
	return allocatedMiB, allocationTime, nil

}

func MemoryAllocationCanaryHealthPerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*canaryHealthPerfParam)
	preARC := multivm.ARCFromPre(pre)

	err := updateLimit(ctx)
	if err != nil {
		s.Fatal("Update of the open file limit failed: ", err)
	}

	iterationsStr, ok := s.Var(iterationsVar)
	var iterations int
	if ok {
		iterationsConv, err := strconv.Atoi(iterationsStr)
		if err != nil {
			s.Fatal("Could not convert the iterations arg to integer: ", err)
		}
		iterations = iterationsConv
	} else {
		iterations = 5
	}

	basemem, err := metrics.NewBaseMemoryStats(ctx, preARC)
	if err != nil {
		s.Fatal("Failed to retrieve base memory stats: ", err)
	}

	var totalMib int64 = 0
	var totalTime time.Duration = 0

	for i := 0; i < iterations; i++ {
		mib, time, err := stressCanary(ctx, s, param, pre.Chrome, preARC)
		if err != nil {
			s.Fatal("Error in the canary stress test: ", err)
		}
		s.Logf("Allocation speed: %f MiB / ms", float64(mib)/float64(time.Milliseconds()))
		totalTime += time
		totalMib += mib
	}
	averageAllocationSpeed := float64(totalMib) / float64(totalTime.Milliseconds())
	s.Logf("Average allocation speed: %f MiB / ms", averageAllocationSpeed)
	averageAllocatedMiB := float64(totalMib) / float64(iterations)
	s.Logf("Average allocated memory: %f MiB", averageAllocatedMiB)

	memoryStats := perf.NewValues()
	if err := metrics.LogMemoryStats(ctx, basemem, preARC, memoryStats, s.OutDir(), ""); err != nil {
		s.Error("Failed to collect memory metrics: ", err)
	}

	nouploadPath := path.Join(s.OutDir(), "noupload")
	if err := os.Mkdir(nouploadPath, 0777); err != nil {
		s.Error("Failed to create a directory for memory metrics: ", err)
	}
	if err := memoryStats.Save(nouploadPath); err != nil {
		s.Error("Failed to save memory metrics: ", err)
	}
	p := perf.NewValues()
	p.Set(
		perf.Metric{
			Name:      "allocatedMiB",
			Unit:      "MiB",
			Direction: perf.SmallerIsBetter,
		},
		averageAllocatedMiB,
	)
	p.Set(
		perf.Metric{
			Name:      "allocationSpeed",
			Unit:      "MiB-per-ms",
			Direction: perf.BiggerIsBetter,
		},
		averageAllocationSpeed,
	)
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf.Values: ", err)
	}
}
