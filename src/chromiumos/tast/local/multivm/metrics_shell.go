// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/memory/metrics"
)

// BaseMetrics is a thin wrapper around metrics.BaseMetrics.
// This is only added to allow tests in other Depots to temporarily compile okay with this change,
// which renames multivm.BaseMetrics into metrics.BaseMetrics.
// TODO(raging):  delete this class and this whole file when no code depends on it anymore.
type BaseMetrics struct {
	innerMetrics *metrics.BaseMemoryStats
}

// NewBaseMetrics is a thin wrapper around metrics.NewBaseMetrics, scheduled for deletion.
// See details in BaseMetrics comment.
func NewBaseMetrics() (*BaseMetrics, error) {
	m, err := metrics.NewBaseMemoryStats()
	if err != nil {
		return nil, err
	}
	return &BaseMetrics{innerMetrics: m}, nil
}

// MemoryMetrics is a thin wrapper around metrics.MemoryMetrics, scheduled for deletion.
// See details in BaseMetrics comment.
func MemoryMetrics(ctx context.Context, base *BaseMetrics, pre *PreData, p *perf.Values, outdir, suffix string) error {
	var innerBase *metrics.BaseMemoryStats
	if base != nil {
		innerBase = base.innerMetrics
	}
	return metrics.LogMemoryStats(ctx, innerBase, ARCFromPre(pre), p, outdir, suffix)
}
