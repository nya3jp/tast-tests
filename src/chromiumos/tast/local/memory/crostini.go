// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"path"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
)

func crostiniLocalPath(exe string) string {
	return path.Join("/home/testuser", exe)
}

const crostiniAllocAllocExe = "memory.Alloc.anon_crostini"

// NewCrostiniAnonAlloc creates an Alloc that allocates anonymous memory in
// Crostini.
func NewCrostiniAnonAlloc(ctx context.Context, cont *vm.Container, ratio float64, oomScoreAdj int64) (Alloc, error) {
	cmd := cont.Command(
		ctx,
		crostiniLocalPath(crostiniAllocAllocExe),
		strconv.FormatFloat(ratio, 'f', 8, 64),
		strconv.FormatInt(oomScoreAdj, 10),
	)
	// NB: vm.Containter.Command sets Stdin because it is required by vsh. But
	// NewAllocator sets Stdin, so clear Stdin so we don't get errors when
	// NewAllocator sets it.
	cmd.Stdin = nil
	return NewCmdAlloc(cmd)
}

const crostiniLimitReclaimExe = "memory.Limit.reclaim_crostini"

// NewCrostiniReclaimLimit creates a Limit that reports how close Crostini is to
// starting reclaim, and OOMing.
func NewCrostiniReclaimLimit(ctx context.Context, cont *vm.Container) (Limit, error) {
	cmd := cont.Command(
		ctx,
		crostiniLocalPath(crostiniLimitReclaimExe),
	)
	// NB: vm.Containter.Command sets Stdin because it is required by vsh. But
	// NewAllocator sets Stdin, so clear Stdin so we don't get errors when
	// NewAllocator sets it.
	cmd.Stdin = nil
	return NewCmdLimit(cmd)
}

// CrostiniData returns the list of all data dependencies needed to run memory
// tests on Crostini.
func CrostiniData() []string {
	return []string{
		crostiniAllocAllocExe,
		crostiniLimitReclaimExe,
	}
}

// CopyCrostiniExes copies all the exes used to implement Alloc and Limit.
func CopyCrostiniExes(ctx context.Context, cont *vm.Container, dataPathGetter func(string) string) error {
	// TODO: cleanup?
	for _, exe := range CrostiniData() {
		if err := cont.PushFile(
			ctx,
			dataPathGetter(exe),
			crostiniLocalPath(exe),
		); err != nil {
			return errors.Wrapf(err, "failed to copy %q to Crostini", exe)
		}
		if err := cont.Command(ctx, "chmod", "755", crostiniLocalPath(exe)).Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrapf(err, "failed to chmod %q", crostiniLocalPath(exe))
		}
	}
	return nil
}
