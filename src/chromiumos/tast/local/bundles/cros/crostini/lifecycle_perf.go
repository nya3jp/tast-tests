// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

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
		Func:         LifecyclePerf,
		Desc:         "Tests process stuff memory kills etc. blah blah",
		Contacts:     []string{"cwd@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		Timeout:      60 * time.Minute,
		Data:         append([]string{crostini.ImageArtifact}, memory.CrostiniData()...),
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
		Vars:         []string{"keepState"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				ExtraHardwareDeps: crostini.CrostiniStable,
			},
		},
	})
}

func LifecyclePerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	defer crostini.RunCrostiniPostTest(ctx, pre)
	cont := pre.Container

	crostininNearOOMLimit, err := memory.NewCrostiniReclaimLimit(ctx, cont)
	if err != nil {
		s.Fatal("Failed to create CrostiniReclaimLimit: ", err)
	}

	// Define the list of processes to launch.
	const numProc = 100
	if err := memory.CopyCrostiniExes(ctx, cont, s.DataPath); err != nil {
		s.Fatal("Failed to copy Crostini memory tools: ", err)
	}
	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}

	// Processes are sized so that if all were alive at once, they would use 2x
	// the memory of the system.
	allocMiB := (2 * int64(info.Total) / numProc) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var procsAliveTasks []memoryuser.KillableTask
	for i := 0; i < numProc; i++ {
		task := memoryuser.NewCrostiniLifecycleTask(cont, i, allocMiB*memory.MiB, 0.67, 1000, crostininNearOOMLimit)
		tasks = append(tasks, task)
		procsAliveTasks = append(procsAliveTasks, task)
	}

	// Define a metric for the number of those processes still alive.
	tasks = append(
		tasks,
		memoryuser.NewStillAliveMetricTask(procsAliveTasks, "procs_alive"),
	)

	rp := &memoryuser.RunParameters{
		ExistingChrome: pre.Chrome,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
