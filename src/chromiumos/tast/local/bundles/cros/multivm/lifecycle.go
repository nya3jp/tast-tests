// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/local/multivm"
	"chromiumos/tast/testing"
)

type lifecycleParam struct {
	inHost, inARC bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         Lifecycle,
		Desc:         "Create many Apps, Tabs, Processes across multiple VMs, and see how many can stay alive",
		Contacts:     []string{"cwd@google.com", "cros-platform-kernel-core@google.com"},
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
			ExtraSoftwareDeps: []string{"arc"},
		}, {
			Name:              "arc_host",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inARC: true, inHost: true},
			ExtraSoftwareDeps: []string{"arc"},
			ExtraData: []string{
				memoryuser.AllocPageFilename,
				memoryuser.JavascriptFilename,
			},
		}, {
			Name:              "host_with_bg_arc",
			Pre:               multivm.ArcStarted(),
			Val:               &lifecycleParam{inHost: true},
			ExtraSoftwareDeps: []string{"arc"},
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
	numTypes := 0
	if param.inHost {
		server = memoryuser.NewMemoryStressServer(s.DataFileSystem())
		numTypes++
	}
	if param.inARC {
		numTypes++
	}

	// Created tabs/apps/etc. should have memory that is a bit compressible.
	// We use the same value as the low compress ratio in
	// platform.MemoryStressBasic.
	const compressRatio = 0.67
	const numTasks = 100
	taskAllocMiB := (2 * int64(info.Total) / numTasks) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var tabsAliveTasks []memoryuser.KillableTask
	var appsAliveTasks []memoryuser.KillableTask
	for i := 0; i < numTasks/numTypes; i++ {
		if param.inHost {
			task := server.NewMemoryStressTask(int(taskAllocMiB), compressRatio, hostLimit)
			tabsAliveTasks = append(tabsAliveTasks, task)
			tasks = append(tasks, task)
		}
		if param.inARC {
			task := memoryuser.NewArcLifecycleTask(len(appsAliveTasks), int64(taskAllocMiB)*memory.MiB, compressRatio, hostLimit)
			appsAliveTasks = append(appsAliveTasks, task)
			tasks = append(tasks, task)
		}
		if len(tasks) == 0 {
			s.Fatal("No MemoryTasks created")
		}
	}

	if param.inHost {
		task := memoryuser.NewStillAliveMetricTask(tabsAliveTasks, "tabs_alive")
		tasks = append(tasks, task)
	}
	if param.inARC {
		task := memoryuser.NewStillAliveMetricTask(appsAliveTasks, "apps_alive")
		tasks = append(tasks, task)
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
