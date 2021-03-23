// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     PerfCallgraph,
		Desc:     "Checks that callchains can be profiled using perf",
		Contacts: []string{"chromeos-kernel-test@google.com"},
		// Call stacks can't currently be unwound on ARM due to the
		// Thumb and ARM ISAs using different registers for the frame pointer.
		SoftwareDeps: []string{"amd64"},
		Attr:         []string{"group:mainline"},
	})
}

func PerfCallgraph(ctx context.Context, s *testing.State) {
	const exe = "/usr/local/libexec/tast/helpers/local/cros/kernel.PerfCallgraph.graph"

	td, err := ioutil.TempDir("", "tast.kernel.PerfCallgraph")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(td)

	trace := filepath.Join(td, "trace")
	if err := testexec.CommandContext(ctx, "perf", "record", "-N", "-e", "cycles", "-g",
		"-o", trace, "--", exe).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to record trace: ", err)
	}

	out, err := testexec.CommandContext(ctx, "perf", "report", "-D",
		"-i", trace).Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatal("Failed to generate report: ", err)
	}
	const outFile = "report.txt"
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), outFile), out, 0644); err != nil {
		s.Error("Failed to write report: ", err)
	}

	// Try to find a sample with a callchain of the expected length.
	// Samples are formatted as follows and separated by blank lines:
	//
	// 41522248799882 0x7e60 [0x70]: PERF_RECORD_SAMPLE(IP, 0x2): 12923/12923: 0x5a385056d84f period: 492431 addr: 0
	// ... FP chain: nr:8
	// .....  0: fffffffffffffe00
	// .....  1: 00005a385056d84f
	// .....  2: 00005a385056d892
	// .....  3: 00005a385056d8e5
	// .....  4: 00005a385056d912
	// .....  5: 00005a385056d991
	// .....  6: 00007a3ddb542ad4
	// .....  7: 00005a385056d6ea
	//  ... thread: kernel.PerfCall:12923
	//  ...... dso: /usr/local/libexec/tast/helpers/local/cros/kernel.PerfCallgraph.graph
	chainRegexp := regexp.MustCompile(`\bFP chain: nr:(\d+)\b`)
	const wantedChainLength = 3
	for _, sample := range strings.Split(string(out), "\n\n") {
		// Skip non-samples and samples for other DSOs.
		if !strings.Contains(sample, "PERF_RECORD_SAMPLE") ||
			!strings.Contains(sample, "dso: "+exe) {
			continue
		}

		// Extract the chain length.
		ms := chainRegexp.FindStringSubmatch(sample)
		if ms == nil {
			continue
		}
		if chainLength, err := strconv.Atoi(ms[1]); err != nil {
			s.Fatalf("Failed to parse callchain length %q: %v", ms[1], err)
		} else if chainLength >= wantedChainLength {
			s.Log("Found callchain of length ", chainLength)
			return
		}
	}
	s.Errorf("Failed to find callchain of length %d or greater; see %v", wantedChainLength, outFile)
}
