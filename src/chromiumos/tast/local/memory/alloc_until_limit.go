// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"fmt"
	"math"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// AllocLimit pairs an Alloc with a Limit, so that we know when to stop
// allocating.
type AllocLimit struct {
	Alloc Alloc
	Limit Limit
	Name  string
}

func distanceToLow(ctx context.Context, globalLimit, limit Limit) (int64, error) {
	var result int64 = math.MaxInt64
	if globalLimit == nil && limit == nil {
		return 0, errors.New("no Limit provided")
	}
	if globalLimit != nil {
		distance, err := globalLimit.Distance(ctx)
		if err != nil {
			return 0, err
		}
		if distance < result {
			result = distance
		}
	}
	if limit != nil {
		distance, err := limit.Distance(ctx)
		if err != nil {
			return 0, err
		}
		if distance < result {
			result = distance
		}
	}
	return result, nil
}

func allocationSize(distance int64) int64 {
	// Allocate 1/4 of the distance, so we don't easily overshoot the Limit.
	distance = distance / 4
	// Truncate to the nearest MiB, because allocation tools expect round MiBs.
	distance = (distance / MiB) * MiB
	// Always allocate at least 1 MiB, so that we slowly overshoot the limit
	// when very near.
	if distance < MiB {
		return MiB
	}
	return distance
}

func freeSize(distance int64) int64 {
	// NB: distance will be negative because it is an allocation size.
	// Truncate to the nearest MiB, because allocation tools expect round MiBs.
	distance = ((distance - MiB + 1) / MiB) * MiB
	// Always free at least 1 MiB, so that we get back under the limit when very
	// near.
	if distance > -MiB {
		return -MiB
	}
	return distance
}

// AllocUntilLimit allocates memory balanced between the passed allocLimits,
// until every Limit is reached.
func AllocUntilLimit(
	ctx context.Context,
	attempts int,
	globalLimit Limit,
	allocLimits ...AllocLimit,
) (allocated [][]float64, errResult error) {
	// TODO: check for OOMs in ChromeOsS.
	allocated = make([][]float64, len(allocLimits))
	testing.ContextLog(ctx, "Allocations starting")
	logTitle := "attempt\t"
	for _, a := range allocLimits {
		logTitle += fmt.Sprintf("\t%s", a.Name)
	}
	testing.ContextLog(ctx, logTitle)
	for attempt := 0; attempt < attempts; attempt++ {
		// Allocate until memory is low everywhere.
		allLow := false
		for !allLow {
			allLow = true
			for _, a := range allocLimits {
				distance, err := distanceToLow(ctx, globalLimit, a.Limit)
				if err != nil {
					return nil, err
				}
				if distance < 0 {
					break
				}
				allLow = false
				if err := a.Alloc.Allocate(ctx, allocationSize(distance)); err != nil {
					return nil, err
				}
			}
		}
		// Free until memory is not low anywhere.
		noneLow := false
		for !noneLow {
			noneLow = true
			for _, a := range allocLimits {
				distance, err := distanceToLow(ctx, globalLimit, a.Limit)
				if err != nil {
					return nil, err
				}
				if distance > 0 {
					break
				}
				noneLow = false
				if err := a.Alloc.Allocate(ctx, freeSize(distance)); err != nil {
					return nil, err
				}
			}
		}
		logRow := fmt.Sprintf("%2d", attempt)
		for i, a := range allocLimits {
			sample := float64(a.Alloc.Allocated()) / float64(MiB)
			allocated[i] = append(allocated[i], sample)
			logRow += fmt.Sprintf("\t%.0f", sample)
		}

		testing.ContextLog(ctx, logRow)
		const attemptInterval = time.Second
		if err := testing.Sleep(ctx, attemptInterval); err != nil {
			return nil, err
		}
	}
	testing.ContextLog(ctx, "Allocations done")
	return allocated, nil
}
