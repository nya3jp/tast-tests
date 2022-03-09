// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"fmt"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/resourced"
	"chromiumos/tast/testing"
)

// ChromeOSAvailableMetrics logs performance metrics for ChromeOS memory margins
// and ChromeOS's available memory counter.
func ChromeOSAvailableMetrics(ctx context.Context, p *perf.Values, suffix string) error {
	rm, err := resourced.NewClient(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Skipping available metrics due to inability to contact resourced: ", err)
		return nil
	}
	margins, err := rm.MemoryMarginsKB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get ChromeOS memory margins")
	}
	available, err := rm.AvailableMemoryKB(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get ChromeOS available memory")
	}
	p.Set(
		perf.Metric{
			Name: fmt.Sprintf("chromeos_margin_critical%s", suffix),
			Unit: "MiB",
		},
		float64(margins.CriticalKB)/KiB,
	)
	p.Set(
		perf.Metric{
			Name: fmt.Sprintf("chromeos_margin_moderate%s", suffix),
			Unit: "MiB",
		},
		float64(margins.ModerateKB)/KiB,
	)
	p.Set(
		perf.Metric{
			Name: fmt.Sprintf("chromeos_available%s", suffix),
			Unit: "MiB",
		},
		float64(available)/KiB,
	)
	return nil
}
