// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LifecycleAndroidPerf,
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
		Timeout: 10 * time.Minute,
	})
}

func LifecycleAndroidPerf(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(arc.PreData)

	// Construct memory.Limit that will throttle tab creation.
	nearOOM, err := memory.NewPageReclaimLimit()
	if err != nil {
		s.Fatal("Failed to make page reclaim Limit: ", err)
	}
	crosCrit, err := memory.NewAvailableCriticalLimit()
	if err != nil {
		s.Fatal("Failed to make ChromeOS available Limit: ", err)
	}
	limit := memory.NewCompositeLimit(nearOOM, crosCrit)

	if err := memoryuser.InstallArcLifecycleTestApps(ctx, pre.ARC, 30); err != nil {
		s.Fatal("Failed to install ArcLifecycleTest apps: ", err)
	}

	// Define the list of tabs to load.
	var tasks []memoryuser.MemoryTask = nil
	for i := 0; i < 30; i++ {
		tasks = append(tasks, memoryuser.NewArcLifecycleTask(i, 200*memory.MiB, limit))
	}

	// Define a metric for the number of those tabs killed.
	var extraPerfMetrics = func(ctx context.Context, testEnv *memoryuser.TestEnv, p *perf.Values, label string) {
		tabsKilled := 0
		for _, task := range tasks {
			killable, ok := task.(memoryuser.KillableTask)
			if ok && !killable.StillAlive(ctx, testEnv) {
				tabsKilled++
			}
		}
		totalTabKillMetric := perf.Metric{
			Name:      "tast_total_tab_kill_" + label,
			Unit:      "count",
			Direction: perf.SmallerIsBetter,
		}
		p.Set(totalTabKillMetric, float64(tabsKilled))
	}

	rp := &memoryuser.RunParameters{
		ExistingChrome:   pre.Chrome,
		ExistingARC:      pre.ARC,
		ExtraPerfMetrics: extraPerfMetrics,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
