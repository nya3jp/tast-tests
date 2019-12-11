// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MemoryAndroidPerf,
		Desc: "How much memory can be allocated in Android before available in android is under 200MB",
		Contacts: []string{
			"cwd@chromium.org",
			"tast-users@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "android_both"},
		Data:         []string{"ArcMemoryAllocatorTest.apk"},
		Params: []testing.Param{{
			Name: "arc", // TODO (cwd): when we have arc.VMBooted, split this into arcvm and arcpp, b/141884011
			Pre:  arc.Booted(),
		}},
		Timeout: 120 * time.Minute,
	})
}

func MemoryAndroidPerf(ctx context.Context, s *testing.State) {
	var (
		allocatedMetric = perf.Metric{Name: "allocated", Unit: "MB", Direction: perf.BiggerIsBetter, Multiple: true}
		marginMetric    = perf.Metric{Name: "critical_margin", Unit: "MB"}
	)

	var a = memory.NewAndroidMemoryAllocator(s.PreValue().(arc.PreData).ARC)

	margin, err := memory.ChromeOSCriticalMargin()
	if err != nil {
		s.Fatal("Failed to read critical margin: ", err)
	}

	p := perf.NewValues()
	p.Set(marginMetric, float64(margin))

	cleanup, err := a.Prepare(ctx, func(p string) string { return s.DataPath(p) })
	if err != nil {
		s.Fatal("Failed to setup ArcMemoryAllocatorTest app: ", err)
	}
	defer cleanup()

	if allocated, err := a.AllocateUntil(
		ctx,
		1*time.Second,
		60,
		margin-5,
	); err != nil {
		s.Fatal("Failed to allocate to critical margin: ", err)
	} else {
		for _, x := range allocated {
			p.Append(allocatedMetric, float64(x)/1048576.0)
		}
	}
	if err := a.FreeAll(ctx); err != nil {
		s.Fatal("Failed to free memory: ", err)
	}
	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}
