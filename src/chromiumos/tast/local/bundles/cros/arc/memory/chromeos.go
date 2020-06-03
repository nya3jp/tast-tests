// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file contains helper functions to allocate memory on ChromeOS.

// Package memory contains common utilities to allocate memory and read memory
// pressure state on ChromeOS and Android.
package memory

import (
	"container/list"
	"context"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory/pressure"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// ChromeOSAllocator helps test code allocate memory on ChromeOS.
type ChromeOSAllocator struct {
	allocated *list.List
	size      uint
}

// NewChromeOSAllocator creates a helper to allocate memory on ChromeOS.
// Returns the helper.
func NewChromeOSAllocator() *ChromeOSAllocator {
	return &ChromeOSAllocator{
		allocated: list.New(),
		size:      0,
	}
}

// Size returns the size of all allocated memory
func (c *ChromeOSAllocator) Size() uint {
	return c.size
}

// Allocate some memory on ChromeOS.
// Parameter size is the size of the allocation in bytes.
// Allocated memory is filled with random data so that page compression can't
// shrink it.
func (c *ChromeOSAllocator) Allocate(size int) error {
	buffer, err := syscall.Mmap(
		-1,
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS,
	)
	if err != nil {
		return errors.Wrapf(err, "unable to allocate %d byte chunk", size)
	}
	// Fill each page with random bytes so that page compression can't reduce
	// the size.
	for i := 0; i < size; i += len(randomPage) {
		copy(buffer[i:], randomPage[:])
	}
	c.allocated.PushBack(buffer)
	c.size += uint(len(buffer))
	return nil
}

// FreeLast frees the most recently allocated buffer.
// Returns the size of the buffer freed.
func (c *ChromeOSAllocator) FreeLast() (int, error) {
	if c.allocated.Len() == 0 {
		return 0, errors.New("nothing to free")
	}
	buffer := c.allocated.Remove(c.allocated.Back()).([]byte)
	size := len(buffer)
	c.size -= uint(size)

	if err := syscall.Munmap(buffer); err != nil {
		return 0, errors.Wrap(err, "unable to free buffer")
	}
	return size, nil
}

// FreeAll frees all allocated buffers.
// Returns the size of freed memory.
func (c *ChromeOSAllocator) FreeAll() (uint, error) {
	size := c.size
	for c.allocated.Len() > 0 {
		if _, err := c.FreeLast(); err != nil {
			return 0, errors.Wrap(err, "unable to free")
		}
	}
	if c.size != 0 {
		return 0, errors.Errorf("allocated size is %d after freeing averything", c.size)
	}
	return size, nil
}

// max returns the larger of two integers.
func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// min returns the smaller of two integers.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

var zoneinfoRE = regexp.MustCompile(`(?m)^Node +\d+, +zone +[^ ]+
(?:(?: +pages free +(\d+)
 +min +(\d+)
 +low +(\d+)
)|(?: +.*
))*`)

// readMinDistanceToZoneMin reads the smallest distance between a zone's free
// count and its min watermark. Small or empty zones are ignored.
// returns the distance in MiB.
func readMinDistanceToZoneMin() (int, error) {
	const zoneInfoFile = "/proc/zoneinfo"
	data, err := ioutil.ReadFile(zoneInfoFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open zoneinfo")
	}

	matches := zoneinfoRE.FindAllStringSubmatch(string(data), -1)
	if matches == nil {
		return 0, errors.Wrap(err, "failed to parse zoneinfo")
	}

	distance := 0
	found := false
	for _, match := range matches {
		free, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return 0, errors.Wrap(err, "failed to parse zone free")
		}
		minWatermark, err := strconv.ParseUint(match[2], 10, 64)
		if err != nil {
			return 0, errors.Wrap(err, "failed to parse zone min")
		}
		lowWatermark, err := strconv.ParseUint(match[3], 10, 64)
		if err != nil {
			return 0, errors.Wrap(err, "failed to parse zone low")
		}
		// Ignore small or empty zones, we don't want to throttle allocations
		// based on a small distance from a small or empty zone.
		const smallZoneLimit = 1000
		if lowWatermark > smallZoneLimit {
			if !found {
				distance = int(free - minWatermark)
				found = true
			} else {
				distance = min(distance, int(free-minWatermark))
			}
		}
	}
	if !found {
		return 0, errors.Wrap(err, "no non-empty zones found")
	}
	const pagesPerMiB = (1024 * 1024) / 4096
	return distance / pagesPerMiB, nil
}

const (
	oomKillMessage   = "Out of memory: Kill process"
	oomSyslogTimeout = 10 * time.Second
)

func checkForOOMs(ctx context.Context, reader *syslog.Reader) error {
	_, err := reader.Wait(ctx, oomSyslogTimeout, func(e *syslog.Entry) bool {
		return strings.Contains(e.Content, oomKillMessage)
	})
	if err == syslog.ErrNotFound {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to check for OOM")
	}
	return errors.New("test triggered Linux OOM killer")
}

// AllocateUntil allocates memory until available memory is at the passed
// margin.  To allow the system to stabalize, it will try attempts times,
// waiting attemptInterval duration between each attempt.
// If too much memory has been allocated, then the extra is freed between
// attempts to avoid overshooting the margin.
// Returns the allocated memory at every attempt.
func (c *ChromeOSAllocator) AllocateUntil(
	ctx context.Context,
	attemptInterval time.Duration,
	attempts int,
	margin uint,
) ([]uint, error) {
	// Create a reader to scan for OOMs, we can't use syslog.Program tofilter to
	// a specific process name because ARCVM includes the PID in the process
	// name field.
	reader, err := syslog.NewReader()
	if err != nil {
		return nil, errors.Wrap(err, "failed to open syslog reader")
	}
	defer reader.Close()

	allocated := make([]uint, attempts)
	for attempt := 0; attempt < attempts; {
		available, err := pressure.Available()
		if err != nil {
			return nil, errors.Wrap(err, "unable to read available")
		}
		aboveMin, err := readMinDistanceToZoneMin()
		if err != nil {
			return nil, errors.Wrap(err, "unable to read distance above zone min threshold")
		}
		// aboveMinDivisor is the fraction the free memory in zone info above
		// the min watermark we should try to allocate at once.
		const aboveMinDivisor = 4
		// allocAvailableDivisor is the fraction of available memory we should
		// try to allocate at once.
		const allocAvailableDivisor = 10
		const bytesInMiB = 1024 * 1024
		if available >= margin && aboveMin/aboveMinDivisor > 0 {
			// Limit buffer size to be a fraction of available memory, and also
			// a fraction of the free memory in the most depleted zone.
			bufferSize := max(int(available-margin)/allocAvailableDivisor, 1)
			bufferSize = min(bufferSize, int(aboveMin/aboveMinDivisor))
			err = c.Allocate(bufferSize * bytesInMiB)
			if err != nil {
				return nil, errors.Wrap(err, "unable to allocate")
			}
		} else {
			allocated[attempt] = c.Size()
			attempt++
			// Available is less than target margin, but it might be much less
			// if the system becomes unresponsive from the memory pressure we
			// are applying. Available memory can drop much faster than the
			// amount allocated, causing us to overshoot and apply much higher
			// memory pressure than intended. To reduce the risk of having the
			// linux OOM killer kill anything, we free anything extra we may
			// have allocated.
			// NB: margin-available should always be small, so it's safe to use
			// an int.
			for toFree := int(margin-available) * bytesInMiB; toFree > 0 && c.Size() > 0; {
				bufferSize, err := c.FreeLast()
				if err != nil {
					return nil, errors.Wrap(err, "unable to free after overshoot")
				}
				toFree -= bufferSize
			}
			if err := testing.Sleep(ctx, attemptInterval); err != nil {
				return nil, errors.Wrap(err, "failed to sleep after allocation attempt")
			}
		}
	}
	if err := checkForOOMs(ctx, reader); err != nil {
		return nil, err
	}
	return allocated, nil
}
