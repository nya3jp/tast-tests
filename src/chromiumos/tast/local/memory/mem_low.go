// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memory contains test helpers for allocating memory and determining
// if memory is low.
package memory

import (
	"context"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// MemLow allows tests to determine if memory is low without having to know the
// specific memory counters used.
type MemLow interface {
	// Distance returns the amount of memory that can be allocated in MB before
	// memory is low. If negative, abs(Distance()) MB must be freed for memory
	// to not be low.
	Distance(ctx context.Context) (int64, error)
}

type chromeOSCriticalMemLow struct {
	criticalMargin int64
}

// readFirstInt64 reads the first unsigned integer from a file.
func readFirstInt64(f string) (int64, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read file %q", f)
	}
	firstString := strings.Split(strings.TrimSpace(string(data)), " ")[0]
	firstInt64, err := strconv.ParseInt(firstString, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return firstInt64, nil
}

// Distance returns the difference between available memory and the critical
// margin, in MB. Result will be negative if available memory is below the
// critical margin.
func (ml *chromeOSCriticalMemLow) Distance(_ context.Context) (int64, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	available, err := readFirstInt64(availableMemorySysFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read ChromeOS available memory")
	}
	return available - ml.criticalMargin, nil
}

// CriticalMargin returns the value of ChromeOS available memory below which
// tabs are killed.
func CriticalMargin() (int64, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	criticalMargin, err := readFirstInt64(marginMemorySysFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read ChromeOS critical margin")
	}
	return criticalMargin, nil
}

// NewChromeOSMemLow creates a MemLow that measures how far away ChromeOS
// available memory is from a specified margin.
func NewChromeOSMemLow(margin int64) (MemLow, error) {
	return &chromeOSCriticalMemLow{margin}, nil
}

// NewChromeOSCriticalMemLow creates a MemLow that measure how far away ChromeOS
// is from 5 MB within the critical memoury pressure margin. Allocators using
// this MemLow as a target will then maintain critical memory pressure.
func NewChromeOSCriticalMemLow() (MemLow, error) {
	criticalMargin, err := CriticalMargin()
	if err != nil {
		return nil, err
	}
	return &chromeOSCriticalMemLow{criticalMargin}, nil
}

type nearOOMMemLow struct {
	largeZones map[string]bool
}

// Distance returns the smallest distance from a zone's free counter to half-way
// between its min and low watermark. If <= 0, this means that page reclaim has
// started and we are at risk of the Linux OOM Killer waking up.
func (ml *nearOOMMemLow) Distance(_ context.Context) (int64, error) {
	infos, err := ReadZoneInfo()
	if err != nil {
		return 0, errors.Wrap(err, "failed to read zone counters")
	}
	var minDistance int64 = math.MaxInt64
	for _, info := range infos {
		if _, ok := ml.largeZones[info.Name]; !ok {
			// Zone is not a large zone.
			continue
		}
		distance := int64(info.Free) - int64((info.Low+info.Min)/2)
		if distance < minDistance {
			minDistance = distance
		}
	}
	if minDistance == math.MaxInt64 {
		return 0, errors.New("no large zones found")
	}
	const bytesInMB = 1024 * 1024
	return minDistance / bytesInMB, nil
}

// NewNearOOMMemLow creates a MemLow that measures how far away ChromeOS is from
// any Linux memory zone's free pages being half-way between the min and low
// watermarks. The intent is to trigger page reclaim by being below the low
// watermark, but keep away from the low watermark to avoid invoking the Linux
// OOM killer.
func NewNearOOMMemLow() (MemLow, error) {
	infos, err := ReadZoneInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read zone counters")
	}
	const largeZoneMinMin = 4194304
	largeZones := make(map[string]bool)
	for _, info := range infos {
		if info.Min > largeZoneMinMin {
			largeZones[info.Name] = true
		}
	}
	if len(largeZones) == 0 {
		return nil, errors.New("no large zones found")
	}
	return &nearOOMMemLow{largeZones}, nil
}

type compositeMemLow struct {
	memLows []MemLow
}

// Distance returns the minimum Distance returned by any child MemLow.
func (ml *compositeMemLow) Distance(ctx context.Context) (int64, error) {
	if len(ml.memLows) == 0 {
		return 0, errors.New("empty composite MemLow")
	}
	minDistance, err := ml.memLows[0].Distance(ctx)
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(ml.memLows); i++ {
		distance, err := ml.memLows[i].Distance(ctx)
		if err != nil {
			return 0, err
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance, nil
}

// NewCompositeMemLow creates a MemLow that returns the minimum Distance()
// returned by all the passed MemLow.
func NewCompositeMemLow(memLows ...MemLow) MemLow {
	return &compositeMemLow{memLows}
}
