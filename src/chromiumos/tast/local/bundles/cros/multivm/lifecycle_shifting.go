// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/bundles/cros/multivm/stats"
	"chromiumos/tast/local/memory"
	arcMemory "chromiumos/tast/local/memory/arc"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type lifecycleShiftingParam struct {
	inHost, inARC, inCrostini bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LifecycleShifting,
		Desc:         "Create many Apps, Tabs, Processes in turn across multiple VMs, and see how many can stay alive",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"randSize"},
		Params: []testing.Param{{
			Name:              "arc_host",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleShiftingParam{inARC: true, inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}},
	})
}

func LifecycleShifting(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*lifecycleShiftingParam)
	r := stats.NewRandFromVar(s.Var("randSize"))

	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}

	// Use a PageReclaimLimit to avoid OOMing in the host. Will be composed with
	// VM limits so that we don't OOM in the host or any VM.
	hostLimit := memory.NewPageReclaimLimit()

	var server *memoryuser.MemoryStressServer
	if param.inHost {
		server = memoryuser.NewMemoryStressServer(s.DataFileSystem())
	}

	var arcLimit memory.Limit
	if param.inARC {
		limit := arcMemory.NewPageReclaimLimit(pre.ARC)
		arcLimit = memory.NewCompositeLimit(hostLimit, limit)
	}

	// Each task allocates 2% of memory.
	taskAllocMiB := (2 * int64(info.Total) / 100) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var appsAliveTasks []memoryuser.KillableTask
	var tabsAliveTasks []memoryuser.KillableTask
	var appsAliveMetrics []*memoryuser.StillAliveMetricTask
	var tabsAliveMetrics []*memoryuser.StillAliveMetricTask
	for i := 0; i < 3; i++ {
		// Each iteration starts 50 tasks.
		const numTasks = 50
		var appTasks []memoryuser.MemoryTask
		var tabTasks []memoryuser.MemoryTask
		for len(appTasks)+len(tabTasks) < numTasks {
			if param.inHost {
				task := server.NewMemoryStressTask(int(stats.ExponentialInt64(taskAllocMiB, r)), 0.67, hostLimit)
				tabTasks = append(tabTasks, task)
				tabsAliveTasks = append(tabsAliveTasks, task)
			}
			if param.inARC && len(appTasks)+len(tabTasks) < numTasks {
				task := memoryuser.NewArcLifecycleTask(len(appsAliveTasks), stats.ExponentialInt64(taskAllocMiB, r)*memory.MiB, 0.67, arcLimit)
				appTasks = append(appTasks, task)
				appsAliveTasks = append(appsAliveTasks, task)
			}
			if len(appTasks) == 0 || len(tabTasks) == 0 {
				s.Fatal("Some MemoryTasks not created")
			}
		}
		// Compute metrics at the end of each iteration.
		tasks = append(tasks, tabTasks...)
		if param.inHost {
			tabsAliveMetric := memoryuser.NewStillAliveMetricTask(
				tabsAliveTasks,
				fmt.Sprintf("tabs_alive_%d", i),
			)
			tabsAliveMetrics = append(tabsAliveMetrics, tabsAliveMetric)
			tasks = append(tasks, tabsAliveMetric)
		}
		tasks = append(tasks, appTasks...)
		if param.inARC {
			appsAliveMetric := memoryuser.NewStillAliveMetricTask(
				appsAliveTasks,
				fmt.Sprintf("apps_alive_%d", i),
			)
			appsAliveMetrics = append(appsAliveMetrics, appsAliveMetric)
			tasks = append(tasks, appsAliveMetric)
		}
	}
	// TODO: compute tabs_alive_min, apps_alive_min. Need a new memoryuser task type?

	// Install Android test apps.
	if param.inARC {
		if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, len(appsAliveTasks)); err != nil {
			s.Fatal("Failed to install ArcLifecycleTestApps: ", err)
		}
	}

	// Run all the tasks.
	rp := &memoryuser.RunParameters{
		UseARC:         pre.ARC != nil,
		ExistingChrome: pre.Chrome,
		ExistingARC:    pre.ARC,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
