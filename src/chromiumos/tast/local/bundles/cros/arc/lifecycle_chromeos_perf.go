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
			ExtraSoftwareDeps: []string{"android_p"},
			Pre:               arc.Booted(),
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
		Timeout: 10 * time.Minute,
	})
}

func LifecycleChromeOSPerf(ctx context.Context, s *testing.State) {
	info, err := kernelmeter.MemInfo()
	if err != nil {
		s.Fatal("Failed to get /proc/meminfo: ", err)
	}

	// Define the list of tabs to load.
	const (
		NumTabs   = 20
		BytesInMB = 1024 * 1024
	)
	var tasks []memoryuser.MemoryTask
	tabAlloc := (int)((info.Total * 2) / (BytesInMB * NumTabs))
	server := memoryuser.NewMemoryStressServer(s.DataFileSystem())
	for i := 0; i < NumTabs; i++ {
		tasks = append(tasks, server.NewMemoryStressTask(tabAlloc, 0.67))
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
