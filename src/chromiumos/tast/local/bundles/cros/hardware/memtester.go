// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hardware

import (
	"context"
	"os"
	"path/filepath"
	"strconv"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Memtester,
		Desc: "Runs memtester to find memory subsystem faults",
		Contacts: []string{
			"puthik@chromium.org", // Original Autotest author
			"derat@chromium.org",  // Tast port author
			"tast-users@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func Memtester(ctx context.Context, s *testing.State) {
	const (
		sizeMB      = 10
		numLoops    = 1
		outFilename = "memtester.txt"
	)

	f, err := os.Create(filepath.Join(s.OutDir(), outFilename))
	if err != nil {
		s.Fatal("Failed to open output file: ", err)
	}
	defer f.Close()

	s.Logf("Testing %d MiB and writing stdout to %s", sizeMB, outFilename)
	cmd := testexec.CommandContext(ctx, "memtester", strconv.Itoa(sizeMB)+"M", strconv.Itoa(numLoops))
	cmd.Stdout = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("memtester failed: ", err)
	}
}
