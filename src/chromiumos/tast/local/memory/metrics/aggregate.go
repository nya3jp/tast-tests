// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics

import (
	"fmt"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/memory"
	memoryarc "chromiumos/tast/local/memory/arc"
)

// ReportSummaryMetrics combines metrics taken from various sources, computes
// and logs aggregate metrics.
func ReportSummaryMetrics(vmEnabled bool, hostSummary *memory.HostSummary, vmSummary *memoryarc.VMSummary, zramSummary *memory.ZramSummary, p *perf.Values, suffix string) {

	totalCachedKernel := vmSummary.CachedKernel
	if vmEnabled {
		totalCachedKernel += hostSummary.HostCachedKernel
	}
	total := float64(hostSummary.MemTotal - hostSummary.MemFree - totalCachedKernel)

	// This may be negative.
	zramSavings := int64(zramSummary.OrigDataSize) - int64(zramSummary.MemUsedTotal)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("total_memory_used%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		total,
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("total_memory_used_noswap%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		total+float64(zramSavings),
	)

	if crosvmArc := hostSummary.CategoryMetrics["crosvm_arcvm"]; crosvmArc != nil {
		p.Set(
			perf.Metric{
				Name:      fmt.Sprintf("crosvm_overhead%s", suffix),
				Unit:      "KiB",
				Direction: perf.SmallerIsBetter,
			},
			float64(crosvmArc.Pss+crosvmArc.PssSwap-crosvmArc.PssGuest),
		)
	}
}
