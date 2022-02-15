// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"math"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/resourced"
)

// Limit allows tests to determine if memory use is close to a limit without
// having to know the specific memory counters used.
type Limit interface {
	// Distance returns the amount of memory that can be allocated in bytes
	// before the limit is reached. If negative, abs(Distance()) bytes must be
	// freed to go below the limit.
	Distance(ctx context.Context) (int64, error)
	// AssertNotReached returns an error if the limit has been reached. Useful
	// for Polls to get information about which limit was exceeded, and by how
	// much.
	AssertNotReached(ctx context.Context) error
}

// AvailableLimit is a Limit for ChromeOS available memory.
type AvailableLimit struct {
	rm     *resourced.Client
	margin uint64
}

// AvailableLimit conforms to Limit interface.
var _ Limit = (*AvailableLimit)(nil)

// Distance returns the difference between available memory and the provided
// margin, in bytes. Result will be negative if available memory is below the
// margin.
func (l *AvailableLimit) Distance(ctx context.Context) (int64, error) {
	availableKiB, err := l.rm.AvailableMemoryKB(ctx)
	if err != nil {
		return 0, err
	}
	return int64(availableKiB*KiB) - int64(l.margin), nil
}

// AssertNotReached checks that available memory is above the margin.
func (l *AvailableLimit) AssertNotReached(ctx context.Context) error {
	distance, err := l.Distance(ctx)
	if err != nil {
		return err
	}
	if distance <= 0 {
		return errors.Errorf("available memory %d is less than margin %d", distance+int64(l.margin), l.margin)
	}
	return nil
}

// NewAvailableLimit creates a Limit that measures how far away ChromeOS
// available memory is from a specified margin, in bytes.
func NewAvailableLimit(ctx context.Context, rm *resourced.Client, margin uint64) (*AvailableLimit, error) {
	return &AvailableLimit{rm, margin}, nil
}

// ZoneInfoLimit is a Limit that uses /proc/zoneinfo to allow tests to
// allocate enough memory to trigger page reclaim, but not so much memory that
// they OOM.
type ZoneInfoLimit struct {
	readZoneInfo func(context.Context) ([]ZoneInfo, error)
	// lowZones is the set of zones we won't allow to get low.
	lowZones map[string]bool
}

// PageReclaimLimit conforms to Limit interface.
var _ Limit = (*ZoneInfoLimit)(nil)

// IgnoreZone checks if a zone is not used. It checks if the pages free, min, low are all 0.
func IgnoreZone(info ZoneInfo) bool {
	return info.Free == 0 && info.Min == 0 && info.Low == 0
}

// Distance computes how far away from OOMing we are. For each zone, compute
// zoneDistance := (min+low)/2. If any zoneDistance in l.lowZones is negative,
// return the lowest zoneDistance to keep any lowZone away from its min
// watermark.
// If no l.lowZones is negative, return the sum of all zoneDistance to indicate
// how many free pages there are in total before we start getting close to the
// min watermark in any of l.lowZones.
func (l *ZoneInfoLimit) Distance(ctx context.Context) (int64, error) {
	infos, err := l.readZoneInfo(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read zone counters")
	}
	var minDistance int64 = math.MaxInt64
	var sumDistance int64
	for _, info := range infos {
		if IgnoreZone(info) {
			continue
		}
		zoneDistance := int64(info.Free) - int64((info.Low+info.Min)/2)
		sumDistance += zoneDistance
		if _, ok := l.lowZones[info.Name]; ok && zoneDistance < minDistance {
			minDistance = zoneDistance
		}
	}
	if minDistance == math.MaxInt64 {
		return 0, errors.New("no matching zones found")
	}
	if minDistance < 0 {
		return minDistance, nil
	}
	return sumDistance, nil
}

// AssertNotReached checks that no zone has its free pages counter below
// (min+low)/2.
func (l *ZoneInfoLimit) AssertNotReached(ctx context.Context) error {
	infos, err := l.readZoneInfo(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read zone counters")
	}
	for _, info := range infos {
		if _, ok := l.lowZones[info.Name]; !ok {
			// We don't care about this zone.
			continue
		}
		if IgnoreZone(info) {
			continue
		}
		distance := int64(info.Free) - int64((info.Low+info.Min)/2)
		if distance <= 0 {
			return errors.Errorf("zone %q free %d is less than (min+low)/2 (%d+%d)/2", info.Name, info.Free, info.Min, info.Low)
		}
	}
	return nil
}

// NewPageReclaimLimit creates a Limit that returns Distance 0 when Linux is
// reclaiming memory and is close to OOMing.
func NewPageReclaimLimit() *ZoneInfoLimit {
	// NB: We only look at zones DMA and DMA32 because there is probably never
	// going to be a Normal specific page allocation, and if Normal is low but
	// there are still plenty of DMA and DMA32 pages, we're not actually close
	// to OOMing because we'll just fetch pages from the other zones first.
	return NewZoneInfoLimit(
		func(_ context.Context) ([]ZoneInfo, error) { return ReadZoneInfo() },
		map[string]bool{
			"DMA":   true,
			"DMA32": true,
		},
	)
}

// NewZoneInfoLimit creates a limit the returns
func NewZoneInfoLimit(readZoneInfo func(context.Context) ([]ZoneInfo, error), zones map[string]bool) *ZoneInfoLimit {
	return &ZoneInfoLimit{readZoneInfo, zones}
}

// CompositeLimit combines a set of Limits.
type CompositeLimit struct {
	limits []Limit
}

// CompositeLimit conforms to Limit interface.
var _ Limit = (*CompositeLimit)(nil)

// Distance returns the minimum Distance returned by any child Limit.
func (l *CompositeLimit) Distance(ctx context.Context) (int64, error) {
	if len(l.limits) == 0 {
		return 0, errors.New("empty compositeLimit")
	}
	minDistance, err := l.limits[0].Distance(ctx)
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(l.limits); i++ {
		distance, err := l.limits[i].Distance(ctx)
		if err != nil {
			return 0, err
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance, nil
}

// AssertNotReached checks that child Limits are above their limits.
func (l *CompositeLimit) AssertNotReached(ctx context.Context) error {
	if len(l.limits) == 0 {
		return errors.New("empty compositeLimit")
	}
	for i := 1; i < len(l.limits); i++ {
		if err := l.limits[i].AssertNotReached(ctx); err != nil {
			return err
		}
	}
	return nil
}

// NewCompositeLimit creates a Limit that returns the minimum Distance()
// returned by all the passed limits.
func NewCompositeLimit(limits ...Limit) *CompositeLimit {
	return &CompositeLimit{limits}
}
