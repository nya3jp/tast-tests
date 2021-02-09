// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

type lifecycleShiftingParam struct {
	inHost, inCrostini, inARC bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         LifecycleShifting,
		Desc:         "Create many Apps, Tabs, Processes in turn across multiple VMs, and see how many can stay alive",
		Contacts:     []string{"cwd@google.com", "cros-platform-kernel-core@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "arc_host",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleShiftingParam{inARC: true, inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "crostini_host",
			Pre:               multivm.CrostiniStarted(),
			Val:               &lifecycleShiftingParam{inCrostini: true, inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				vm.ArtifactData(),
				crostini.GetContainerMetadataArtifact("buster", false),
				crostini.GetContainerRootfsArtifact("buster", false),
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
			ExtraHardwareDeps: crostini.CrostiniStable,
		}},
	})
}

func LifecycleShifting(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*lifecycleShiftingParam)

	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}

	// Use a PageReclaimLimit to avoid OOMing in the host. Will be composed with
	// VM limits so that we don't OOM in the host or any VM.
	hostLimit := memory.NewPageReclaimLimit()

	var server *memoryuser.MemoryStressServer
	numTypes := 0
	if param.inHost {
		server = memoryuser.NewMemoryStressServer(s.DataFileSystem())
		defer server.Close()
		numTypes++
	}
	if param.inARC {
		numTypes++
	}
	if param.inCrostini {
		numTypes++
	}

	if numTypes == 0 {
		s.Fatal("No lifecycle unit types")
	}
	// Created tabs/apps/etc. should have memory that is a bit compressible.
	// We use the same value as the low compress ratio in
	// platform.MemoryStressBasic.
	const compressRatio = 0.67
	taskAllocMiB := (2 * int64(info.Total) / 100) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var appsAliveTasks []memoryuser.KillableTask
	var tabsAliveTasks []memoryuser.KillableTask
	var procsAliveTasks []memoryuser.KillableTask
	var appsAliveMetrics []*memoryuser.StillAliveMetricTask
	var tabsAliveMetrics []*memoryuser.StillAliveMetricTask
	var procsAliveMetrics []*memoryuser.StillAliveMetricTask
	for i := 0; i < 3; i++ {
		const numTasks = 50
		if param.inHost {
			for j := 0; j < numTasks/numTypes; j++ {
				task := server.NewMemoryStressTask(int(taskAllocMiB), compressRatio, hostLimit)
				tabsAliveTasks = append(tabsAliveTasks, task)
				tasks = append(tasks, task)
			}
			task := memoryuser.NewStillAliveMetricTask(
				tabsAliveTasks,
				fmt.Sprintf("tabs_alive_%d", i),
			)
			tabsAliveMetrics = append(tabsAliveMetrics, task)
			tasks = append(tasks, task)
		}
		if param.inARC {
			for j := 0; j < numTasks/numTypes; j++ {
				task := memoryuser.NewArcLifecycleTask(len(appsAliveTasks), taskAllocMiB*memory.MiB, compressRatio, hostLimit)
				appsAliveTasks = append(appsAliveTasks, task)
				tasks = append(tasks, task)
			}
			task := memoryuser.NewStillAliveMetricTask(
				appsAliveTasks,
				fmt.Sprintf("apps_alive_%d", i),
			)
			appsAliveMetrics = append(appsAliveMetrics, task)
			tasks = append(tasks, task)
		}
		if param.inCrostini {
			for j := 0; j < numTasks/numTypes; j++ {
				task := memoryuser.NewCrostiniLifecycleTask(pre.Crostini, len(procsAliveTasks), taskAllocMiB, compressRatio, hostLimit)
				procsAliveTasks = append(procsAliveTasks, task)
				tasks = append(tasks, task)
			}
			task := memoryuser.NewStillAliveMetricTask(
				procsAliveTasks,
				fmt.Sprintf("procs_alive_%d", i),
			)
			procsAliveMetrics = append(procsAliveMetrics, task)
			tasks = append(tasks, task)
		}
	}

	if param.inHost {
		task := memoryuser.NewMinStillAliveMetricTask(tabsAliveMetrics, "tabs_alive_min")
		tasks = append(tasks, task)
	}
	if param.inARC {
		task := memoryuser.NewMinStillAliveMetricTask(appsAliveMetrics, "apps_alive_min")
		tasks = append(tasks, task)
		if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, len(appsAliveTasks)); err != nil {
			s.Fatal("Failed to install ArcLifecycleTestApps: ", err)
		}
	}
	if param.inCrostini {
		task := memoryuser.NewMinStillAliveMetricTask(procsAliveMetrics, "procs_alive_min")
		tasks = append(tasks, task)
		if err := memoryuser.InstallCrostiniLifecycle(ctx, pre.Crostini); err != nil {
			s.Fatal("Failed to install Crostini lifecycle unit: ", err)
		}
		defer func() {
			if err := memoryuser.UninstallCrostiniLifecycle(ctx, pre.Crostini); err != nil {
				s.Error("Failed to uninstall Crostini lifecycle unit: ", err)
			}
		}()
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
