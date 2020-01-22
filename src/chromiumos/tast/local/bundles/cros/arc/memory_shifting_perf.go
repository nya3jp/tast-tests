// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/memory"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryShiftingPerf,
		Desc: "Alternate applying memory pressure to ChromeOS and Android",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"ArcMemoryAllocatorTest.apk"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android"},
			Pre:               arc.Booted(),
			// TODO(b/146081124): Reenable the test when this test stops hanging ARC++ devices.
			ExtraAttr: []string{"disabled"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Pre:               arc.VMBooted(),
		}},
		Timeout: 20 * time.Minute,
	})
}

// mean returns the arithmetic mean of a slice of integers.
func mean(values []int64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += float64(v)
	}
	return sum / float64(len(values))
}

func MemoryShiftingPerf(ctx context.Context, s *testing.State) {
	minAndroidMetric := perf.Metric{Name: "min_android", Unit: "MiB", Direction: perf.BiggerIsBetter}
	maxAndroidMetric := perf.Metric{Name: "max_android", Unit: "MiB", Direction: perf.BiggerIsBetter}
	minChromeOSMetric := perf.Metric{Name: "min_chromeos", Unit: "MiB", Direction: perf.BiggerIsBetter}
	maxChromeOSMetric := perf.Metric{Name: "max_chromeos", Unit: "MiB", Direction: perf.BiggerIsBetter}
	marginMetric := perf.Metric{Name: "critical_margin", Unit: "MiB", Direction: perf.SmallerIsBetter}
	p := perf.NewValues()
	margin, err := memory.ChromeOSCriticalMargin()
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}
	p.Set(marginMetric, float64(margin))
	a := memory.NewAndroidAllocator(s.PreValue().(arc.PreData).ARC)
	cleanup, err := a.Prepare(ctx, func(p string) string { return s.DataPath(p) })
	if err != nil {
		s.Fatal("Failed to setup ArcMemoryAllocatorTest app: ", err)
	}
	defer cleanup()
	c := memory.NewChromeOSAllocator()
	arcMin := math.MaxFloat64
	arcMax := 0.0
	crosMin := math.MaxFloat64
	crosMax := 0.0
	for shift := 0; shift < 3; shift++ {
		// Allocate in Android.
		const epsilon = 5 // We want to be consistently under the critical margin, so make the target available just inside.
		arcAllocated, err := a.AllocateUntil(
			ctx,
			time.Second,
			30,
			margin-epsilon,
		)
		if err != nil {
			s.Fatal("Failed to allocate Android memory: ", err)
		}
		// Use the last 10 attempts to get stable results.
		arcMean := mean(arcAllocated[len(arcAllocated)-10 : len(arcAllocated)])
		arcMin = math.Min(arcMin, arcMean)
		arcMax = math.Max(arcMax, arcMean)
		if err := a.FreeAll(ctx); err != nil {
			s.Fatal("Failed to free Android memory: ", err)
		}

		// Allocate in ChromeOS.
		crosAllocated, err := c.AllocateUntil(
			ctx,
			time.Second,
			30,
			margin-epsilon,
		)
		if err != nil {
			s.Fatal("Failed to allocate ChromeOS memory: ", err)
		}
		// Use the last 10 attempts to get stable results.
		crosMean := mean(crosAllocated[len(crosAllocated)-10 : len(crosAllocated)])
		crosMin = math.Min(crosMin, crosMean)
		crosMax = math.Max(crosMax, crosMean)
		if _, err := c.FreeAll(); err != nil {
			s.Fatal("Failed to free ChromeOS memory: ", err)
		}
	}
	const bytesInMiB = 1024 * 1024
	p.Set(minAndroidMetric, float64(arcMin)/bytesInMiB)
	p.Set(maxAndroidMetric, float64(arcMax)/bytesInMiB)
	p.Set(minChromeOSMetric, float64(crosMin)/bytesInMiB)
	p.Set(maxChromeOSMetric, float64(crosMax)/bytesInMiB)
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
