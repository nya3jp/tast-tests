// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multivm

import (
	"context"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/memory/metrics"
)

// BaseMetrics is a thin wrapper around metrics.BaseMetrics
// this is only added to allow tests in other Depots to temporarily compile okay with this change,
// which renames multivm.BaseMetrics into metrics.BaseMetrics
// TODO(raging):  delete this class and this whole file when no code depends on it anymore
type BaseMetrics struct {
	innerMetrics *metrics.BaseMetrics
}

// NewBaseMetrics is a thin wrapper around metrics.NewBaseMetrics, scheduled for deletion
// - see details in BaseMetrics comment
func NewBaseMetrics() (*BaseMetrics, error) {
	m, err := metrics.NewBaseMetrics()
	if err != nil {
		return nil, err
	}
	return &BaseMetrics{innerMetrics: m}, nil
}

// MemoryMetrics is a thin wrapper around metrics.MemoryMetrics, scheduled for deletion
// - see details in BaseMetrics comment
func MemoryMetrics(ctx context.Context, base *BaseMetrics, pre *PreData, p *perf.Values, outdir, suffix string) error {
	var innerBase *metrics.BaseMetrics
	if base != nil {
		innerBase = base.innerMetrics
	}
	return metrics.MemoryMetrics(ctx, innerBase, ARCFromPre(pre), p, outdir, suffix)
}
