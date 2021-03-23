// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/memory"
)

// NewPageReclaimLimit creates a memory.Limit that measures if ARC is reclaiming
// pages and is near to OOMing.
func NewPageReclaimLimit(a *arc.ARC) *memory.ZoneInfoLimit {
	// NB: We only look at zones DMA and DMA32 because there is probably never
	// going to be a Normal specific page allocation, and if Normal is low but
	// there are still plenty of DMA and DMA32 pages, we're not actually close
	// to OOMing because we'll just fetch pages from the other zones first.
	return memory.NewZoneInfoLimit(
		func(ctx context.Context) ([]memory.ZoneInfo, error) {
			data, err := a.Command(ctx, "cat", "/proc/zoneinfo").Output(testexec.DumpLogOnError)
			if err != nil {
				return nil, errors.Wrap(err, "failed to cat ARC /proc/zoneinfo")
			}
			return memory.ParseZoneInfo(string(data))
		},
		map[string]bool{
			"DMA":   true,
			"DMA32": true,
		},
	)
}
