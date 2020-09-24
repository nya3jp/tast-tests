// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Alloc is an interface implemented by memory allocators so that memory tests
// can abstract away different types/locations of memory allocations.
type Alloc interface {
	// Allocate allocates or frees memory. If bytes is positive memory is
	// allocated, freed if bytes is negative. Implementations are allowed to
	// round bytes to any internally specified size before allocating.
	Allocate(ctx context.Context, bytes int64) error
	// Allocated returns the total number of bytes allocated by this Alloc.
	Allocated() uint64
	// Close frees all memory and other resources associated with this Alloc.
	Close(ctx context.Context) error
}

type alloc struct {
	cmdWithPipes
	allocated uint64
}

var _ Alloc = (*alloc)(nil)

// Allocate allocates megs MB of memory. If megs is negative, that much memory
// is freed.
func (a *alloc) Allocate(ctx context.Context, megs int64) error {
	if err := a.writeLine(ctx, strconv.FormatInt(megs, 10)); err != nil {
		return errors.Wrap(err, "failed to write allocation size")
	}
	line, err := a.readLine(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to read total allocation size")
	}
	allocated, err := strconv.ParseUint(line, 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse allocation total")
	}
	a.allocated = allocated
	return nil
}

// Allocated returnes the total number of bytes allocated by this command.
func (a *alloc) Allocated() uint64 {
	return a.allocated
}

// Close terminates the allocator command, freeing any allocated memory.
func (a *alloc) Close(ctx context.Context) error {
	if err := a.writeLine(ctx, ""); err != nil {
		return errors.Wrap(err, "failed to write empty line")
	}
	return a.cmd.Wait(testexec.DumpLogOnError)
}

// NewAllocator creates an Allocator from a passed testexec.Cmd. The Cmd reads
// allocation sizes in bytes, one per line with negative size meaning free. The
// Cmd then allocates/frees the memory and prints the total number of bytes
// allocated followed by a newline.
func NewAllocator(cmd *testexec.Cmd) (Alloc, error) {
	cmdWithPipes, err := newCmdWithPipes(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create allocator")
	}
	return &alloc{cmdWithPipes, 0}, nil
}
