// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/memory"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryChromeOSPerf,
		Desc: "How much memory can be allocated in ChromeOS before critical memory pressure",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "android_both"},
		Params: []testing.Param{{
			Name: "noarc",
			Pre:  chrome.LoggedIn(),
		}, {
			Name:              "arc",
			Pre:               arc.Booted(),
			ExtraSoftwareDeps: []string{"android"},
		}, {
			Name:              "arcvm",
			Pre:               arc.VMBooted(),
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 10 * time.Minute,
	})
}

func MemoryChromeOSPerf(ctx context.Context, s *testing.State) {
	allocatedMetric := perf.Metric{Name: "allocated", Unit: "MiB", Direction: perf.BiggerIsBetter, Multiple: true}
	marginMetric := perf.Metric{Name: "critical_margin", Unit: "MiB"}
	margin, err := memory.ChromeOSCriticalMargin()
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}
	p := perf.NewValues()
	p.Set(marginMetric, float64(margin))
	c := memory.NewChromeOSAllocator()
	const epsilon = 5 // We want to be consistently under the critical margin, so make the target available just inside.
	allocated, err := c.AllocateUntil(
		ctx,
		time.Second,
		60,
		margin-epsilon,
	)
	if err != nil {
		s.Fatal("Failed to allocate to critical margin: ", err)
	}
	for _, a := range allocated {
		const bytesInMiB = 1024 * 1024
		p.Append(allocatedMetric, float64(a)/bytesInMiB)
	}
	if _, err := c.FreeAll(); err != nil {
		s.Fatal("Failed to free allocated memory: ")
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
