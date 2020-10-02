// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LifecyclePerf,
		Desc: "Launch many memory hogging apps, and count how many are killed",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		Data: []string{
			memoryuser.AllocPageFilename,
			memoryuser.JavascriptFilename,
		},
		Params: []testing.Param{{
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.Booted(),
		}},
		Timeout: 60 * time.Minute,
	})
}

func LifecyclePerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(arc.PreData)

	// Construct memory.Limit that will throttle tab creation.
	nearOOM, err := memory.NewPageReclaimLimit()
	if err != nil {
		s.Fatal("Failed to make page reclaim Limit: ", err)
	}
	// TODO: android near oom
	crosCrit, err := memory.NewAvailableCriticalLimit()
	if err != nil {
		s.Fatal("Failed to make ChromeOS available Limit: ", err)
	}
	limit := memory.NewCompositeLimit(nearOOM, crosCrit)

	// Define the list of tabs to load.
	const numApps = 100
	if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, numApps); err != nil {
		s.Fatal("Failed to install ArcLifecycleTest apps: ", err)
	}
	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}
	// Tabs are sized so that if all tabs were alive at once, they would use 2x
	// the memory of the system.
	appAllocMiB := (2 * int64(info.Total) / numApps) / memory.MiB
	var tasks []memoryuser.MemoryTask
	var appsAliveTasks []memoryuser.KillableTask
	var appsRunTasks []memoryuser.SilentFailTask
	for i := 0; i < numApps; i++ {
		task := memoryuser.NewBestEffortArcLifecycleTask(i, appAllocMiB*memory.MiB, 0.67, limit)
		tasks = append(tasks, task)
		appsAliveTasks = append(appsAliveTasks, task)
		appsRunTasks = append(appsRunTasks, task)
	}

	tasks = append(
		tasks,
		memoryuser.NewStillAliveMetricTask(appsAliveTasks, "apps_alive"),
		memoryuser.NewRunSucceededMetricTask(appsRunTasks, "apps_run"),
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
