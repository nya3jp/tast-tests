// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LifecycleArcCrostiniPerf,
		Desc:         "Tests process stuff memory kills etc. blah blah",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      60 * time.Minute,
		Data:         append([]string{crostini.ImageArtifact}, memory.CrostiniData()...),
		Pre:          crostini.StartedARCEnabled(),
		HardwareDeps: crostini.CrostiniStable,
		SoftwareDeps: []string{"chrome", "vm_host", "android_vm"},
		Vars:         []string{"keepState"},
	})
}

func LifecycleArcCrostiniPerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, pre)
	cont := pre.Container

	// TODO Construct memory.Limit that will throttle tab creation.
	crostininNearOOMLimit, err := memory.NewCrostiniReclaimLimit(ctx, cont)
	if err != nil {
		s.Fatal("Failed to create CrostiniReclaimLimit: ", err)
	}

	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}

	// Define the list of processes to launch.
	const numTask = 100

	// Tasks are sized so that if all were alive at once, they would use 2x
	// the memory of the system.
	allocMiB := (2 * int64(info.Total) / numTask) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var arcTasks []memoryuser.MemoryTask
	var crostiniTasks []memoryuser.MemoryTask
	for i := 0; i < numTask; i++ {
		var task memoryuser.MemoryTask
		switch i % 2 {
		case 0:
			task = memoryuser.NewBestEffortArcLifecycleTask(len(arcTasks), allocMiB*memory.MiB, 0.67, nil)
			arcTasks = append(arcTasks, task)
		case 1:
			task = memoryuser.NewCrostiniLifecycleTask(cont, len(crostiniTasks), allocMiB*memory.MiB, 0.67, 1000, crostininNearOOMLimit)
			crostiniTasks = append(crostiniTasks, task)
		default:
			s.Fatal("Error in creating tasks")
		}
		tasks = append(tasks, task)
	}

	if err := memory.CopyCrostiniExes(ctx, cont, s.DataPath); err != nil {
		s.Fatal("Failed to copy Crostini memory tools: ", err)
	}
	if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, len(arcTasks)); err != nil {
		s.Fatal("Failed to install ArcLifecycleTest apps: ", err)
	}

	// Define a metric for the number of those tabs killed.
	var extraPerfMetrics = func(ctx context.Context, testEnv *memoryuser.TestEnv, p *perf.Values, label string) {
		appsAlive := 0
		appsLaunched := 0
		for _, task := range arcTasks {
			killable, ok := task.(memoryuser.KillableTask)
			if ok && killable.StillAlive(ctx, testEnv) {
				appsAlive++
			}
			bestEffort, ok := task.(memoryuser.BestEffortTask)
			if ok && bestEffort.Succeeded() {
				appsLaunched++
			}
		}
		totalAppAliveMetric := perf.Metric{
			Name:      "apps_alive",
			Unit:      "count",
			Direction: perf.BiggerIsBetter,
		}
		p.Set(totalAppAliveMetric, float64(appsAlive))
		totalAppLaunchedMetric := perf.Metric{
			Name:      "apps_launched",
			Unit:      "count",
			Direction: perf.BiggerIsBetter,
		}
		p.Set(totalAppLaunchedMetric, float64(appsLaunched))

		procsAlive := 0
		for _, task := range tasks {
			killable, ok := task.(memoryuser.KillableTask)
			if ok && killable.StillAlive(ctx, testEnv) {
				procsAlive++
			}
		}
		totalProcAliveMetric := perf.Metric{
			Name:      "procs_alive",
			Unit:      "count",
			Direction: perf.BiggerIsBetter,
		}
		p.Set(totalProcAliveMetric, float64(procsAlive))
	}

	rp := &memoryuser.RunParameters{
		UseARC:           true,
		ExistingChrome:   pre.Chrome,
		ExistingARC:      pre.ARC,
		ExtraPerfMetrics: extraPerfMetrics,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
