// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"fmt"

	"chromiumos/tast/common/perf"
)

// HostSummary captures a few key data items that are used to compute
// overall system memory status.
// All items are expressed in bytes.
type HostSummary struct {
	MemTotal               uint64
	MemFree                uint64
	HostCachedKernel       uint64
	CrosVMParentPss        uint64
	CrosVMChildrenPss      uint64
	CrosVMParentGuestMap   uint64
	CrosVMChildrenGuestMap uint64
}

// VMSummary holds overall information on metrics from a VM.
type VMSummary struct {
	UsedPss      uint64
	CachedKernel uint64
	CachedPss    uint64
}

// ZramSummary holds a summary of ZRAM usage by the host.
type ZramSummary struct {
	OrigDataSize  uint64
	ComprDataSize uint64
	MemUsedTotal  uint64
}

// LogSummaryMetrics combines metrics taken from various sources, computes
// and logs aggregate metrics.
func LogSummaryMetrics(ctx context.Context, vmEnabled bool, hostSummary *HostSummary, vmSummary *VMSummary, zramSummary *ZramSummary, p *perf.Values, suffix string) {
	totalCachedKernel := vmSummary.CachedKernel
	if vmEnabled {
		totalCachedKernel += hostSummary.HostCachedKernel
	}
	total := float64((hostSummary.MemTotal - hostSummary.MemFree - totalCachedKernel) / KiB)

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
		total+float64(zramSavings)/KiB,
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("total_crosvm_host%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64((hostSummary.CrosVMParentPss+hostSummary.CrosVMChildrenPss)/KiB),
	)

	p.Set(
		perf.Metric{
			Name:      fmt.Sprintf("android_userland_footprint%s", suffix),
			Unit:      "KiB",
			Direction: perf.SmallerIsBetter,
		},
		float64((vmSummary.UsedPss+vmSummary.CachedPss+hostSummary.CrosVMChildrenPss-hostSummary.CrosVMChildrenGuestMap)/KiB),
	)
}
