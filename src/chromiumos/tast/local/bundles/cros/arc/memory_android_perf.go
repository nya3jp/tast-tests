// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"sort"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/arc"
	arcMemory "chromiumos/tast/local/bundles/cros/arc/memory"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryAndroidPerf,
		Desc: "How much memory can be allocated in Android before available in android is critical",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         arcMemory.AndroidData(),
		Params: []testing.Param{{
			// For manual testing only, does not run automatically.
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name: "vm",
			// TODO(b:176944540): Re-enable the test once it is fixed, otherwise, delete it.
			//ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 10 * time.Minute,
	})
}

func MemoryAndroidPerf(ctx context.Context, s *testing.State) {
	allocatedMetric := perf.Metric{Name: "allocated", Unit: "MiB", Direction: perf.BiggerIsBetter, Multiple: true}
	allocatedP90Metric := perf.Metric{Name: "allocated_p90", Unit: "MiB", Direction: perf.BiggerIsBetter}
	marginMetric := perf.Metric{Name: "critical_margin", Unit: "MiB"}

	a := arcMemory.NewAndroidAllocator(s.FixtValue().(*arc.PreData).ARC)

	margin, err := memory.CriticalMargin()
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}

	p := perf.NewValues()
	p.Set(marginMetric, float64(margin)/memory.MiB)

	cleanup, err := a.Prepare(ctx, func(p string) string { return s.DataPath(p) })
	if err != nil {
		s.Fatal("Failed to setup ArcMemoryAllocatorTest app: ", err)
	}
	defer cleanup()

	const epsilon = 5 * memory.MiB // We want to be consistently under the critical margin, so make the target available just inside.
	allocated, err := a.AllocateUntil(ctx, time.Second, 60, margin-epsilon)
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

	if err := a.FreeAll(ctx); err != nil {
		s.Fatal("Failed to free memory: ", err)
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
