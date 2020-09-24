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

// CmdAlloc implements the Alloc interface by forwarding allocation requests to
// a remote process.
type CmdAlloc struct {
	cmdWithPipes
	allocated uint64
}

// CmdAlloc conforms to Alloc interface.
var _ Alloc = (*CmdAlloc)(nil)

// Allocate memory. If bytes is negative, that much memory is freed.
func (a *CmdAlloc) Allocate(_ context.Context, bytes int64) error {
	if err := a.writeLine(strconv.FormatInt(bytes, 10)); err != nil {
		return errors.Wrap(err, "failed to write allocation size")
	}
	line, err := a.readLine()
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
func (a *CmdAlloc) Allocated() uint64 {
	return a.allocated
}

// ErrProcessAlreadyExited tells callers of Alloc.Close that the underlying
// process had exited before the call to Close().
var ErrProcessAlreadyExited = errors.New("process has already exited")

// Close terminates the allocator command, freeing any allocated memory.
func (a *CmdAlloc) Close(_ context.Context) error {
	// NB: cmd.ProcessState is filled in by Wait(), so if it exists we've
	// already exited the process.
	if a.cmd.ProcessState != nil {
		return ErrProcessAlreadyExited
	}
	if err := a.writeLine(""); err != nil {
		return errors.Wrap(err, "failed to write empty line")
	}
	return a.wait()
}

// NewCmdAlloc creates an Allocator from a passed testexec.Cmd. The Cmd reads
// allocation sizes in bytes, one per line with negative size meaning free. The
// Cmd then allocates/frees the memory and prints the total number of bytes
// allocated followed by a newline.
func NewCmdAlloc(cmd *testexec.Cmd) (*CmdAlloc, error) {
	cmdWithPipes, err := newCmdWithPipes(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create allocator")
	}
	return &CmdAlloc{cmdWithPipes, 0}, nil
}

// NewAnonAlloc creates an Alloc that allocates anonymous memory in ChromeOS.
func NewAnonAlloc(ctx context.Context, ratio float64) (Alloc, error) {
	const exe = "/usr/libexec/tast/helpers/local/cros/memory.Alloc.anon"
	return NewCmdAlloc(testexec.CommandContext(ctx, exe, strconv.FormatFloat(ratio, 'd', 10, 64)))
}
