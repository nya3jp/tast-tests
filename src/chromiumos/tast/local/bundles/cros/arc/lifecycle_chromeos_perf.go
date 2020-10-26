// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/memory/kernelmeter"
	"chromiumos/tast/local/memory/memoryuser"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LifecycleChromeOSPerf,
		Desc: "Launch many memory hogging tabs, and count how many are killed",
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
			Name:              "noarc",
			ExtraSoftwareDeps: []string{"arc"}, // to prevent this from running on non-ARC boards
			Pre:               chrome.LoggedIn(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.Booted(),
		}},
		Timeout: 10 * time.Minute,
	})
}

func LifecycleChromeOSPerf(ctx context.Context, s *testing.State) {
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

	// Define the list of tabs to load.
	const numTabs = 100
	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}
	// Tabs are sized so that if all tabs were alive at once, they would use 2x
	// the memory of the system.
	tabAllocMiB := (int)((2 * info.Total / numTabs) / memory.MiB)
	var tasks []memoryuser.MemoryTask
	server := memoryuser.NewMemoryStressServer(s.DataFileSystem())
	defer server.Close()
	for i := 0; i < numTabs; i++ {
		tasks = append(tasks, server.NewMemoryStressTask(tabAllocMiB, 0.67, limit))
	}
	s.Logf("Created tasks to open %d tabs of %d MiB", numTabs, tabAllocMiB)

	// Define a metric for the number of those tabs still alive.
	var extraPerfMetrics = func(ctx context.Context, testEnv *memoryuser.TestEnv, p *perf.Values, label string) {
		tabsAlive := 0
		for _, task := range tasks {
			killable, ok := task.(memoryuser.KillableTask)
			if ok && killable.StillAlive(ctx, testEnv) {
				tabsAlive++
			}
		}
		totalTabKillMetric := perf.Metric{
			Name:      "tabs_alive",
			Unit:      "count",
			Direction: perf.BiggerIsBetter,
		}
		p.Set(totalTabKillMetric, float64(tabsAlive))
	}

	var a *arc.ARC
	cr, ok := s.PreValue().(*chrome.Chrome)
	if !ok {
		pre := s.PreValue().(arc.PreData)
		cr = pre.Chrome
		a = pre.ARC
	}
	rp := &memoryuser.RunParameters{
		ExistingChrome:   cr,
		ExistingARC:      a,
		ExtraPerfMetrics: extraPerfMetrics,
	}
	if err := memoryuser.RunTest(ctx, s.OutDir(), tasks, rp); err != nil {
		s.Fatal("RunTest failed: ", err)
	}
}
