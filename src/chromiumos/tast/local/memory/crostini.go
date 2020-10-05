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
	"chromiumos/tast/testing"
)

// NewAnonAlloc creates an Alloc that allocates anonymous memory in ChromeOS.
func NewAnonAlloc(ctx context.Context, ratio float64) (Alloc, error) {
	const exe = "/usr/libexec/tast/helpers/local/cros/memory.Alloc.anon"
	return NewAllocator(testexec.CommandContext(ctx, exe, strconv.FormatFloat(ratio, 'd', 10, 64)))
}

const crostiniAnonAllocExe = "memory.Alloc.anon_crostini"

func crostiniLocalPath(exe string) string {
	return path.Join("/home/testuser", exe)
}

// NewCrostiniAnonAlloc creates an Alloc that allocates anonymous memory in
// Crostini.
func NewCrostiniAnonAlloc(ctx context.Context, cont *vm.Container, ratio float64) (Alloc, error) {
	cmd := cont.Command(
		ctx,
		crostiniLocalPath(crostiniAnonAllocExe),
		strconv.FormatFloat(ratio, 'f', 8, 64),
	)
	// NB: vm.Containter.Command sets Stdin because it is required by vsh. But
	// NewAllocator sets Stdin, so clear Stdin so we don't get errors when
	// NewAllocator sets it.
	cmd.Stdin = nil
	return NewAllocator(cmd)
}

// CrostiniData returns the list of all data dependencies needed to run memory
// tests on Crostini.
func CrostiniData() []string {
	return []string{
		crostiniAnonAllocExe,
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
	data, err := cont.Command(ctx, "ls", "-la", "/home/testuser").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to ls home folder")
	}
	testing.ContextLog(ctx, "Crostini home folder: ", string(data))
	return nil
}
