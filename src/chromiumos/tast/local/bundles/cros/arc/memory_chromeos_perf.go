// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryChromeOSPerf,
		Desc: "How much memory can be allocated in ChromeOS before critical memory pressure",
		Contacts: []string{
			"cwd@chromium.org",
			"tast-users@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "noarc",
			Pre:  chrome.LoggedIn(),
		}, {
			Name:              "arc", // TODO (b/141884011): when we have arc.VMBooted, split this into arcvm and arcpp.
			Pre:               arc.Booted(),
			ExtraSoftwareDeps: []string{"android_both"},
		}},
		Timeout: 10 * time.Minute,
	})
}

func MemoryChromeOSPerf(ctx context.Context, s *testing.State) {
	const bytesInMiB = 1048576.0
	allocatedMetric := perf.Metric{Name: "allocated", Unit: "MB", Direction: perf.BiggerIsBetter, Multiple: true}
	marginMetric := perf.Metric{Name: "critical_margin", Unit: "MB"}
	margin, err := memory.ChromeOSCriticalMargin()
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}
	p := perf.NewValues()
	p.Set(marginMetric, float64(margin))
	c := memory.NewChromeOSAllocator()
	allocated, err := c.AllocateUntil(
		ctx,
		time.Second,
		60,
		margin-5, // NB: we want to be just inside the critical margin.
	)
	if err != nil {
		s.Fatal("Failed to allocate to critical margin: ", err)
	}
	for _, a := range allocated {
		p.Append(allocatedMetric, float64(a)/bytesInMiB)
	}
	if _, err := c.FreeAll(); err != nil {
		s.Fatal("Failed to free allocated memory: ")
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
