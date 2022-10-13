// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/memory/metrics"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type canaryHealthPerfParam struct {
	canary           memoryuser.CanaryType
	allocationTarget memoryuser.AllocationTarget
	browserType      browser.Type
}

const iterationsVar = "multivm.MemoryCanaryPerf.iterations"

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryCanaryPerf,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "How much memory can we allocate before the specified canary dies",
		Contacts: []string{
			"kokiryu@chromium.org",
			"cwd@google.com",
			"arcvm-memory@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "tab_host",
			Pre:  multivm.NoVMStarted(),
			Val:  &canaryHealthPerfParam{memoryuser.Tab, memoryuser.Host, browser.TypeAsh},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_host",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.App, memoryuser.Host, browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.App, memoryuser.Arc, browser.TypeAsh},
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
		}, {
			Name: "tab_host_lacros",
			Pre:  multivm.NoVMLacrosStarted(),
			Val:  &canaryHealthPerfParam{memoryuser.Tab, memoryuser.Host, browser.TypeLacros},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_host_lacros",
			Pre:               multivm.ArcLacrosStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.App, memoryuser.Host, browser.TypeLacros},
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "app_arc_lacros",
			Pre:               multivm.ArcLacrosStarted(),
			Val:               &canaryHealthPerfParam{memoryuser.App, memoryuser.Arc, browser.TypeLacros},
			ExtraSoftwareDeps: []string{"android_vm", "lacros"},
		}},
		Vars: []string{
			iterationsVar,
		},
		Timeout: 30 * time.Minute,
	})
}

const initialOpenFileLimit = 2048

// updateOpenFileLimit updates the soft & hard limits of the number of open files by the test process.
// We need this because this test opens one file (stdin) for one memory allocator.
func updateOpenFileLimit(ctx context.Context, softLimit, hardLimit int) error {
	pid := os.Getpid()
	testing.ContextLogf(ctx, "pid: %d", pid)
	cmd := testexec.CommandContext(ctx, "prlimit", fmt.Sprintf("--pid=%d", pid), fmt.Sprintf("--nofile=%d:%d", softLimit, hardLimit))
	testing.ContextLogf(ctx, "The open file limit is updated to %d:%d", softLimit, hardLimit)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to update the open file limit")
	}
	return nil
}

// getCurrentOpenFileLimit returns the current soft & hard limits of the number of open files by the test process.
func getCurrentOpenFileLimit(ctx context.Context) (int, int, error) {
	pid := os.Getpid()
	softLimitOutputBytes, err := testexec.CommandContext(ctx, "prlimit", fmt.Sprintf("--pid=%d", pid), "-o", "SOFT", "--noheading", "--nofile").Output()
	if err != nil {
		return -1, -1, errors.Wrap(err, "failed to get the soft limit")
	}
	softLimit, err := strconv.Atoi(strings.TrimSpace(string(softLimitOutputBytes)))
	if err != nil {
		return -1, -1, errors.Wrap(err, "failed to parse the soft limit")
	}
	hardLimitOutputBytes, err := testexec.CommandContext(ctx, "prlimit", fmt.Sprintf("--pid=%d", pid), "-o", "HARD", "--noheading", "--nofile").Output()
	if err != nil {
		return -1, -1, errors.Wrap(err, "failed to get the hard limit")
	}
	hardLimit, err := strconv.Atoi(strings.TrimSpace(string(hardLimitOutputBytes)))
	if err != nil {
		return -1, -1, errors.Wrap(err, "failed to parse the hard limit")
	}
	return softLimit, hardLimit, nil
}

const canaryAllocatedMiB = 0
const canaryCompressionRatio = 0.
const singleAllocatorMiB = 50
const allocatorComplessionRatio = 0.67

func stressCanary(ctx context.Context, s *testing.State, param *canaryHealthPerfParam, cr *chrome.Chrome, br *browser.Browser, a *arc.ARC) (int64, time.Duration, error) {
	originalSoftLimit, originalHardLimit, err := getCurrentOpenFileLimit(ctx)
	if err != nil {
		s.Fatal("Failed to get the open file limit: ", err)
	}
	defer updateOpenFileLimit(ctx, originalSoftLimit, originalHardLimit)

	var openFileLimit = initialOpenFileLimit

	err = updateOpenFileLimit(ctx, openFileLimit, openFileLimit)
	if err != nil {
		s.Fatal("Update of the open file limit failed: ", err)
	}

	var canary memoryuser.Canary
	switch param.canary {
	case memoryuser.Tab:
		canary = memoryuser.NewTabCanary(ctx, canaryAllocatedMiB, canaryCompressionRatio, s.DataFileSystem(), br, false)
	case memoryuser.App:
		canary, err = memoryuser.NewAppCanary(ctx, canaryAllocatedMiB, canaryCompressionRatio, cr, a)
		if err != nil {
			return -1, -1, errors.Wrap(err, "failed to create the canary")
		}
	default:
		s.Fatal("Invalid canary type")
	}
	canary.Run(ctx)
	defer canary.Close(ctx)

	target := param.allocationTarget
	allocationManager := memoryuser.NewMemoryAllocationManager(ctx, target, singleAllocatorMiB, allocatorComplessionRatio, a)
	defer allocationManager.Cleanup(ctx)

	var allocationTime time.Duration = 0
	var allocatedMiB int64 = 0

	for {
		// Increase the open file limit when needed.
		if allocationManager.NumOfAllocators() >= openFileLimit/3 {
			openFileLimit *= 2
			updateOpenFileLimit(ctx, openFileLimit, openFileLimit)
		}
		if !canary.StillAlive(ctx) {
			s.Logf("%s died after %d MiB allocations", canary.String(), allocationManager.TotalAllocatedMiB())
			allocatedMiB += allocationManager.TotalAllocatedMiB()
			break
		}
		if err := allocationManager.AssertNoDeadAllocator(); err != nil {
			return -1, -1, errors.Wrap(err, "an allocator is killed before the canary")
		}
		start := time.Now()
		if err := allocationManager.AddAllocator(ctx); err != nil {
			return -1, -1, errors.Wrap(err, "failed to add an allocator")
		}
		elapsed := time.Since(start)
		allocationTime += elapsed
	}
	return allocatedMiB, allocationTime, nil
}

func MemoryCanaryPerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*canaryHealthPerfParam)
	preARC := multivm.ARCFromPre(pre)
	br, cleanupBr, err := browserfixt.SetUp(ctx, pre.Chrome.Chrome(), param.browserType)
	if err != nil {
		s.Fatal("Failed to get Browser: ", err)
	}
	defer cleanupBr(ctx)

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
		mib, time, err := stressCanary(ctx, s, param, pre.Chrome, br, preARC)
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
