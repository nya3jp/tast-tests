// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memtester runs the memtester utility to find memory subsystem faults.
// See http://pyropus.ca/software/memtester/ for more details.
package memtester

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Run executes the memtester utility using the supplied amount of memory and number of iterations.
// The utility's stdout is written to a memtester.txt file in the test output dir.
func Run(ctx context.Context, bytes int64, iters int) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get out dir")
	}
	f, err := os.Create(filepath.Join(outDir, "memtester.txt"))
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "memtester", strconv.FormatInt(bytes, 10)+"B", strconv.Itoa(iters))
	if f != nil {
		cmd.Stdout = f
	}
	return cmd.Run(testexec.DumpLogOnError)
}
