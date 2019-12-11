// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file contains helper functions to allocate memory on ChromeOS

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

func readFirstInt(f string) (int, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	available, err := strconv.Atoi(strings.Split(strings.TrimSpace(string(data)), " ")[0])
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer: ", data)
	}
	return available, nil
}

// ChromeOSAllocator helps test code allocate memory on ChromeOS
type ChromeOSAllocator struct {
	allocated *list.List
	size      int
}

// NewChromeOSAllocator creates a helper to allocate memory on ChromeOS
//
// returns the helper
func NewChromeOSAllocator() *ChromeOSAllocator {
	return &ChromeOSAllocator{
		allocated: list.New(),
		size:      0,
	}
}

// ChromeOSAvailable reads available memory from sysfs
//
// returns available memory in MB
func ChromeOSAvailable() (int, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	return readFirstInt(availableMemorySysFile)
}

// ChromeOSCriticalMargin reads the critical margin from sysfs
//
// returns margin in MB
func ChromeOSCriticalMargin() (int, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	return readFirstInt(marginMemorySysFile)
}

// Size of all allocated memory
func (c *ChromeOSAllocator) Size() int {
	return c.size
}

// Allocate some memory on ChromeOS
// Fills the memory with random data so that page compression can't shrink it
func (c *ChromeOSAllocator) Allocate(size int) error {
	buffer, err := syscall.Mmap(
		-1,
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANONYMOUS,
	)
	if err != nil {
		return errors.Wrapf(err, "Unable to allocate %d byte chunk: ", size)
	}
	// Fill each page with random bytes so that page compression can't reduce
	// the size
	for i := 0; i < size; i += len(randomPage) {
		copy(buffer[i:], randomPage[:])
	}
	c.allocated.PushBack(buffer)
	c.size += len(buffer)
	return nil
}

// FreeLast frees the most recently allocated buffer
//
// returns the size of the buffer freed
func (c *ChromeOSAllocator) FreeLast() (int, error) {
	if c.allocated.Len() == 0 {
		return 0, errors.New("Nothing to free")
	}
	buffer := c.allocated.Remove(c.allocated.Back()).([]byte)
	size := len(buffer)
	c.size -= size

	if err := syscall.Munmap(buffer); err != nil {
		return 0, errors.Wrap(err, "Unable to free buffer: ")
	}
	return size, nil
}

// FreeAll allocated buffers
//
// returns the size of freed memory
func (c *ChromeOSAllocator) FreeAll() (int, error) {
	size := c.size
	for c.allocated.Len() > 0 {
		if _, err := c.FreeLast(); err != nil {
			return 0, errors.Wrap(err, "Unable to free: ")
		}
	}
	if c.size != 0 {
		return 0, errors.Errorf("Allocated size is %d after freeing averything", c.size)
	}
	return size, nil
}

func max(x int, y int) int {
	if x > y {
		return x
	}
	return y
}

const bytesPerMB = 1048576

// AllocateUntil allocates memory until available memory is at the passed
// margin.  To allow the system to stabalize, it will try attempts times,
// waiting attemptTimeout duration between each attempt.
// If too much memory has been allocated, then the extra is freed between
// attempts to avoid overshooting the margin.
//
// returns the allocated memory at every attempt
func (c *ChromeOSAllocator) AllocateUntil(
	ctx context.Context,
	attemptTimeout time.Duration,
	attempts int,
	margin int,
) ([]int, error) {
	allocated := make([]int, attempts)
	for attempt := 0; attempt < attempts; {
		available, err := ChromeOSAvailable()
		if err != nil {
			return nil, errors.Wrap(err, "Unable to read available: ")
		}
		if available >= margin {
			bufferSize := max((available-margin)/10, 1) * bytesPerMB
			err = c.Allocate(bufferSize)
			if err != nil {
				return nil, errors.Wrap(err, "Unable to allocate: ")
			}
		} else {
			allocated[attempt] = c.Size()
			attempt++
			// Available is less than target margin, but it might be much less
			// free anything extra we may have allocated
			for toFree := (margin - available) * bytesPerMB; toFree > 0 && c.Size() > 0; {
				bufferSize, err := c.FreeLast()
				if err != nil {
					return nil, errors.Wrap(err, "Unable to free after overshoot: ")
				}
				toFree -= bufferSize
			}
			testing.Sleep(ctx, attemptTimeout)
		}
	}
	return allocated, nil

}
