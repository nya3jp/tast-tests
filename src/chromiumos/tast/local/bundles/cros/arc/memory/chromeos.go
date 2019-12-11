// Copyright 2019 The Chromium OS Authors. All rights reserved.
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// readFirstInt reads the first integer from a file.
func readFirstInt(f string) (int, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	available, err := strconv.Atoi(strings.Split(strings.TrimSpace(string(data)), " ")[0])
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return available, nil
}

// ChromeOSAllocator helps test code allocate memory on ChromeOS.
type ChromeOSAllocator struct {
	allocated *list.List
	size      int
}

// NewChromeOSAllocator creates a helper to allocate memory on ChromeOS.
// Returns the helper.
func NewChromeOSAllocator() *ChromeOSAllocator {
	return &ChromeOSAllocator{
		allocated: list.New(),
		size:      0,
	}
}

// ChromeOSAvailable reads available memory from sysfs.
// Returns available memory in MB.
func ChromeOSAvailable() (int, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	return readFirstInt(availableMemorySysFile)
}

// ChromeOSCriticalMargin reads the critical margin from sysfs.
// Returns margin in MB.
func ChromeOSCriticalMargin() (int, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	return readFirstInt(marginMemorySysFile)
}

// Size returns the size of all allocated memory
func (c *ChromeOSAllocator) Size() int {
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
	c.size += len(buffer)
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
	c.size -= size

	if err := syscall.Munmap(buffer); err != nil {
		return 0, errors.Wrap(err, "unable to free buffer")
	}
	return size, nil
}

// FreeAll frees all allocated buffers.
// Returns the size of freed memory.
func (c *ChromeOSAllocator) FreeAll() (int, error) {
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

// AllocateUntil allocates memory until available memory is at the passed
// margin.  To allow the system to stabalize, it will try attempts times,
// waiting attemptInterval duration between each attempt.
// If too much memory has been allocated, then the extra is freed between
// attempts to avoid overshooting the margin.
// Returns the allocated memory at every attempt.
func (c *ChromeOSAllocator) AllocateUntil(
	ctx context.Context,
	attemptInterval time.Duration,
	attempts, margin int,
) ([]int, error) {
	allocated := make([]int, attempts)
	for attempt := 0; attempt < attempts; {
		available, err := ChromeOSAvailable()
		if err != nil {
			return nil, errors.Wrap(err, "unable to read available")
		}
		const bytesInMiB = 1024 * 1024
		if available >= margin {
			bufferSize := max((available-margin)/10, 1) * bytesInMiB
			err = c.Allocate(bufferSize)
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
			for toFree := (margin - available) * bytesInMiB; toFree > 0 && c.Size() > 0; {
				bufferSize, err := c.FreeLast()
				if err != nil {
					return nil, errors.Wrap(err, "unable to free after overshoot")
				}
				toFree -= bufferSize
			}
			if err := testing.Sleep(ctx, attemptTimeout); err != nil {
				return nil, errors.Wrap(err, "failed to sleep after allocation attempt")
			}
		}
	}
	return allocated, nil

}
