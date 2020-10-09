// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"
	"time"

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
	var appsAliveTasks []memoryuser.KillableTask
	var appsRunTasks []memoryuser.SilentFailTask
	var procsAliveTasks []memoryuser.KillableTask
	for i := 0; i < numTask; i++ {
		switch i % 2 {
		case 0:
			task := memoryuser.NewBestEffortArcLifecycleTask(len(appsAliveTasks), allocMiB*memory.MiB, 0.67, nil)
			tasks = append(tasks, task)
			appsAliveTasks = append(appsAliveTasks, task)
			appsRunTasks = append(appsRunTasks, task)
		case 1:
			task := memoryuser.NewCrostiniLifecycleTask(cont, len(procsAliveTasks), allocMiB*memory.MiB, 0.67, 1000, crostininNearOOMLimit)
			tasks = append(tasks, task)
			procsAliveTasks = append(procsAliveTasks, task)
		default:
			s.Fatal("Error in creating tasks")
		}
	}

	if err := memory.CopyCrostiniExes(ctx, cont, s.DataPath); err != nil {
		s.Fatal("Failed to copy Crostini memory tools: ", err)
	}
	if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, len(appsAliveTasks)); err != nil {
		s.Fatal("Failed to install ArcLifecycleTest apps: ", err)
	}

	// Define metrics for the number of ARC apps and Crostini processes killed.
	tasks = append(
		tasks,
		memoryuser.NewStillAliveMetricTask(appsAliveTasks, "apps_alive"),
		memoryuser.NewRunSucceededMetricTask(appsRunTasks, "apps_run"),
		memoryuser.NewStillAliveMetricTask(procsAliveTasks, "procs_alive"),
	)

	rp := &memoryuser.RunParameters{
		UseARC:         true,
		ExistingChrome: pre.Chrome,
		ExistingARC:    pre.ARC,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
