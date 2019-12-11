// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryShiftingPerf,
		Desc: "Alternate applying memory pressure to ChromeOS and Android",
		Contacts: []string{
			"cwd@chromium.org",
			"tast-users@chromium.org",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "android_both"},
		Data:         []string{"ArcMemoryAllocatorTest.apk"},
		Params: []testing.Param{{
			Name: "arc", // TODO (b/141884011): when we have arc.VMBooted, split this into arcvm and arcpp.
			Pre:  arc.Booted(),
		}},
		Timeout: 20 * time.Minute,
	})
}

// maxf returns the larger of two floating point numbers.
func maxf(x float64, y float64) float64 {
	if x > y {
		return x
	}
	return y
}

// minf returns the smaller of two floating point numbers.
func minf(x float64, y float64) float64 {
	if x < y {
		return x
	}
	return y
}

// mean returns the arithmetic mean of a slice of integers.
func mean(values []int) float64 {
	sum := 0.0
	for _, v := range values {
		sum += float64(v)
	}
	return sum / float64(len(values))
}

func MemoryShiftingPerf(ctx context.Context, s *testing.State) {
	const bytesInMiB = 1048576.0
	minAndroidMetric := perf.Metric{Name: "min_android", Unit: "MB", Direction: perf.BiggerIsBetter}
	maxAndroidMetric := perf.Metric{Name: "max_android", Unit: "MB", Direction: perf.BiggerIsBetter}
	minChromeOSMetric := perf.Metric{Name: "min_chromeos", Unit: "MB", Direction: perf.BiggerIsBetter}
	maxChromeOSMetric := perf.Metric{Name: "max_chromeos", Unit: "MB", Direction: perf.BiggerIsBetter}
	marginMetric := perf.Metric{Name: "critical_margin", Unit: "MB", Direction: perf.SmallerIsBetter}
	p := perf.NewValues()
	margin, err := memory.ChromeOSCriticalMargin()
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}
	p.Set(marginMetric, float64(margin))
	a := memory.NewAndroidMemoryAllocator(s.PreValue().(arc.PreData).ARC)
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
		arcAllocated, err := a.AllocateUntil(
			ctx,
			time.Second,
			30,
			margin-5, // NB: we want to be just inside the critical margin.
		)
		if err != nil {
			s.Fatal("Failed to allocate Android memory: ", err)
		}
		// Use the last 10 attempts to get stable results.
		arcMean := mean(arcAllocated[len(arcAllocated)-10 : len(arcAllocated)])
		arcMin = minf(arcMin, arcMean)
		arcMax = maxf(arcMax, arcMean)
		if err := a.FreeAll(ctx); err != nil {
			s.Fatal("Failed to free Android memory: ", err)
		}

		// Allocate in ChromeOS.
		crosAllocated, err := c.AllocateUntil(
			ctx,
			time.Second,
			30,
			margin-5, // NB: we want to be just inside the critical margin.
		)
		if err != nil {
			s.Fatal("Failed to allocate ChromeOS memory: ", err)
		}
		// Use the last 10 attempts to get stable results.
		crosMean := mean(crosAllocated[len(crosAllocated)-10 : len(crosAllocated)])
		crosMin = minf(crosMin, crosMean)
		crosMax = maxf(crosMax, crosMean)
		if _, err := c.FreeAll(); err != nil {
			s.Fatal("Failed to free ChromeOS memory: ", err)
		}
	}
	p.Set(minAndroidMetric, float64(arcMin)/bytesInMiB)
	p.Set(maxAndroidMetric, float64(arcMax)/bytesInMiB)
	p.Set(minChromeOSMetric, float64(crosMin)/bytesInMiB)
	p.Set(maxChromeOSMetric, float64(crosMax)/bytesInMiB)
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
