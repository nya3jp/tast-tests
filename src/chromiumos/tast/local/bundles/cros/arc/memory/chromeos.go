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
	"strconv"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/memory"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// readFirstUint reads the first unsigned integer from a file.
func readFirstUint(f string) (uint, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	firstString := strings.Split(strings.TrimSpace(string(data)), " ")[0]
	firstUint, err := strconv.ParseUint(firstString, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return uint(firstUint), nil
}

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
	margin int64,
) ([]uint, error) {
	// Create a reader to scan for OOMs, we can't use syslog.Program tofilter to
	// a specific process name because ARCVM includes the PID in the process
	// name field.
	reader, err := syslog.NewReader(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open syslog reader")
	}
	defer reader.Close()

	crosCrit, err := memory.NewChromeOSMemLow(margin)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make ChromeOS critical MemLow")
	}
	nearOOM, err := memory.NewNearOOMMemLow()
	if err != nil {
		return nil, errors.Wrap(err, "failed to make near OOM MemLow")
	}
	memLow := memory.NewCompositeMemLow(crosCrit, nearOOM)

	allocated := make([]uint, attempts)
	for attempt := 0; attempt < attempts; attempt++ {
		const bytesInMB = 1024 * 1024
		for {
			distance, err := memLow.Distance(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read MemLow distance")
			}
			if distance <= 0 {
				break
			}
			allocMB := distance / 4
			if allocMB == 0 {
				allocMB = 1
			}
			err = c.Allocate(int(allocMB * bytesInMB))
			if err != nil {
				return nil, errors.Wrap(err, "unable to allocate")
			}
		}
		allocated[attempt] = c.Size()
		testing.ContextLogf(ctx, "Attempt %d: %d MB", attempt, c.Size()/bytesInMB)
		// Available is less than target margin, but it might be much less
		// if the system becomes unresponsive from the memory pressure we
		// are applying. Available memory can drop much faster than the
		// amount allocated, causing us to overshoot and apply much higher
		// memory pressure than intended. To reduce the risk of having the
		// linux OOM killer kill anything, we free anything extra we may
		// have allocated.
		for {
			distance, err := memLow.Distance(ctx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to read MemLow distance")
			}
			if distance > 0 {
				break
			}
			if _, err := c.FreeLast(); err != nil {
				return nil, errors.Wrap(err, "unable to free after overshoot")
			}
		}
		if err := testing.Sleep(ctx, attemptInterval); err != nil {
			return nil, errors.Wrap(err, "failed to sleep after allocation attempt")
		}
	}
	if err := checkForOOMs(ctx, reader); err != nil {
		return nil, err
	}
	return allocated, nil
}
