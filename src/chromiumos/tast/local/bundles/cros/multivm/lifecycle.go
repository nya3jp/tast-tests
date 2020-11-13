// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/local/memory"
	arcMemory "chromiumos/tast/local/memory/arc"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type lifecycleParam struct {
	inHost, inARC, inCrostini bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Lifecycle,
		Desc:         "Create many Apps, Tabs, Processes across multiple VMs, and see how many can stay alive",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "host",
			Pre:  multivm.NoVMStarted(),
			Val:  &lifecycleParam{inHost: true},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "arc",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inARC: true},
			ExtraSoftwareDeps: []string{"android_vm"},
		}, {
			Name:              "arc_host",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inARC: true, inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "host_with_bg_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inHost: true},
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}},
	})
}

func Lifecycle(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(*multivm.PreData)
	param := s.Param().(*lifecycleParam)

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

	const numTasks = 100
	taskAllocMiB := (2 * int64(info.Total) / numTasks) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var tabsAliveTasks []memoryuser.KillableTask
	var appsAliveTasks []memoryuser.KillableTask
	for len(tasks) < numTasks {
		if param.inHost {
			task := server.NewMemoryStressTask(int(taskAllocMiB), 0.67, hostLimit)
			tasks = append(tasks, task)
			tabsAliveTasks = append(tabsAliveTasks, task)
		}
		if param.inARC && len(tasks) < numTasks {
			task := memoryuser.NewArcLifecycleTask(len(appsAliveTasks), int64(taskAllocMiB)*memory.MiB, 0.67, arcLimit)
			tasks = append(tasks, task)
			appsAliveTasks = append(appsAliveTasks, task)
		}
		if len(tasks) == 0 {
			s.Fatal("No MemoryTasks created")
		}
	}

	if param.inHost {
		tasks = append(
			tasks,
			memoryuser.NewStillAliveMetricTask(tabsAliveTasks, "tabs_alive"),
		)
	}
	if param.inARC {
		if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, len(appsAliveTasks)); err != nil {
			s.Fatal("Failed to install ArcLifecycleTestApps: ", err)
		}
		tasks = append(
			tasks,
			memoryuser.NewStillAliveMetricTask(appsAliveTasks, "apps_alive"),
		)
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
