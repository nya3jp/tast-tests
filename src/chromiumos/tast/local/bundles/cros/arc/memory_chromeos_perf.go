// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"sort"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/resourced"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryChromeOSPerf,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "How much memory can be allocated in ChromeOS before critical memory pressure",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "noarc",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"arc"}, // to prevent this from running on non-ARC boards
			Fixture:           "chromeLoggedIn",
		}, {
			ExtraSoftwareDeps: []string{"android_p"},
			Fixture:           "arcBooted",
		}, {
			Name:              "vm",
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
			Fixture:           "arcBooted",
		}},
		Timeout: 10 * time.Minute,
	})
}

func MemoryChromeOSPerf(ctx context.Context, s *testing.State) {
	rm, err := resourced.NewClient(ctx)
	if err != nil {
		s.Fatal("Failed to create Resource Manager client: ", err)
	}
	allocatedMetric := perf.Metric{Name: "allocated", Unit: "MiB", Direction: perf.BiggerIsBetter, Multiple: true}
	allocatedP90Metric := perf.Metric{Name: "allocated_p90", Unit: "MiB", Direction: perf.BiggerIsBetter}
	marginMetric := perf.Metric{Name: "critical_margin", Unit: "MiB"}
	margins, err := rm.MemoryMarginsKB(ctx)
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}
	p := perf.NewValues()
	p.Set(marginMetric, float64(margins.CriticalKB)/memory.KiB)
	c := memory.NewChromeOSAllocator()
	defer c.FreeAll()
	const epsilon = 5 * memory.MiB // We want to be consistently under the critical margin, so make the target available just inside.
	allocated, err := c.AllocateUntil(
		ctx,
		rm,
		time.Second,
		60,
		margins.CriticalKB*memory.KiB-epsilon,
	)
	if err != nil {
		s.Fatal("Failed to allocate to critical margin: ", err)
	}
	var allocatedFloat []float64
	for _, a := range allocated {
		const bytesInMiB = 1024 * 1024
		aMiB := float64(a) / bytesInMiB
		allocatedFloat = append(allocatedFloat, aMiB)
		p.Append(allocatedMetric, aMiB)
	}
	sort.Float64s(allocatedFloat)
	p90Index := int(float64(len(allocatedFloat))*0.9) - 1
	p.Set(allocatedP90Metric, allocatedFloat[p90Index])

	if _, err := c.FreeAll(); err != nil {
		s.Fatal("Failed to free allocated memory: ")
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
