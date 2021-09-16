// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PerfETM,
		Desc: "Verify ETM functionality with the perf tool",
		Contacts: []string{
			"denik@chromium.org",
			"c-compiler-chrome@google.com",
		},
		// CoreSight/ETM is the Arm technology.
		SoftwareDeps: []string{"arm"},
		// ETM is the optional HW implemented only on Trogdor SoC.
		HardwareDeps: hwdep.D(hwdep.Platform("trogdor")),
		Attr:         []string{"group:mainline"},
	})
}

// verifyETMIsEnabled checks that CoreSight/ETM is enabled in the kernel.
func verifyETMIsEnabled(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "perf", "list")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	perfListFile := filepath.Join(s.OutDir(), "perf-list.txt")
	if err = ioutil.WriteFile(perfListFile, out, 0644); err != nil {
		s.Errorf("Write to %q failed: %v", perfListFile, err)
	}
	if !strings.Contains(string(out), "cs_etm") {
		// Make sure CoreSight is listed in the Kernel PMU events.
		s.Fatal("CoreSight/ETM is not enabled on the device")
	}
}

// verifyETMData verifies that the report contains an AUX record with ETM data.
func verifyETMData(s *testing.State, report string) {
	etmDataRegexp := regexp.MustCompile(`CoreSight ETM Trace data: size (\d+) bytes`)
	records := strings.Split(report, "\n\n")
	for _, record := range records {
		match := etmDataRegexp.FindStringSubmatch(record)
		if match == nil {
			continue
		}
		if size, err := strconv.Atoi(match[1]); err != nil || size == 0 {
			continue
		}
		s.Logf("ETM buffer %q", match[0])
		return
	}
	s.Error("Couldn't find AUX buffer in perf report")
}

// verifyLastBranchSamples verifies that the report contains a last branch sample.
func verifyLastBranchSamples(s *testing.State, report, tracedCommand string) {
	sampleRecordRegexp := regexp.MustCompile("PERF_RECORD_SAMPLE")
	branchStackSizeRegexp := regexp.MustCompile(`branch stack: nr:(\d+)`)
	threadRegexp := regexp.MustCompile(`thread: (\S+):\d+`)
	records := strings.Split(report, "\n\n")
	numberOfRecords := 0
	for _, record := range records {
		sampleMatch := sampleRecordRegexp.FindString(record)
		if sampleMatch == "" {
			continue
		}
		numberOfRecords++
		bsSizeMatch := branchStackSizeRegexp.FindStringSubmatch(record)
		if bsSizeMatch == nil {
			continue
		}
		var size int
		var err error
		if size, err = strconv.Atoi(bsSizeMatch[1]); err != nil || size == 0 {
			continue
		}
		threadMatch := threadRegexp.FindStringSubmatch(record)

		if threadMatch != nil && (tracedCommand == "" || tracedCommand == threadMatch[1]) {
			// We are ok with any last branch record if no tracedCommand is passed.
			s.Logf("Found a sample with %q, stack size %v", threadMatch[1], size)
			return
		}
		// Record is either invalid or belongs to a different command.
		// Continue the search.
	}
	s.Error("Couldn't find a valid Last Branch sample")
	s.Error("Total number of samples: ", numberOfRecords)
}

// perfETMPerThread records ETM trace in per-thread mode and verifies the raw dump.
func perfETMPerThread(ctx context.Context, s *testing.State) {
	const tracedCommand = "ls"
	perfData := filepath.Join(s.OutDir(), "per-thread-perf.data")

	// Test ETM profile collection.
	// -m ,1M reduces ETM data down to 1MB regardless of workload and execution time.
	// -N doesn't clutter HOME directory with unnecessary debug data.
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cs_etm/@tmc_etr0/", "-N", "-m", ",1M", "-o", perfData, "--per-thread", tracedCommand)
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the raw profile dump.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfData)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("-------------------------------------")
	s.Log("Verifying ETM data in per-thread mode")
	verifyETMData(s, string(out))

	// Test ETM trace decoding and sample synthesis.
	perfInjectData := filepath.Join(s.OutDir(), "per-thread-perf-inject.data")
	// --strip reduces the output size.
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=i1000il", "-i", perfData, "-o", perfInjectData, "--strip")
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("Verifying Last Branch samples in per-thread mode")
	verifyLastBranchSamples(s, string(out), tracedCommand)
}

// perfETMSystemWide records ETM trace in system-wide mode and verifies the raw dump.
func perfETMSystemWide(ctx context.Context, s *testing.State) {
	const tracedCommand = "ls"
	perfData := filepath.Join(s.OutDir(), "system-wide-perf.data")

	// Test ETM profile collection.
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cs_etm/@tmc_etr0/uk", "-m", ",1M", "-N", "-o", perfData, "-a", tracedCommand)
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the raw profile dump.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfData)
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("--------------------------------------")
	s.Log("Verifying ETM data in system-wide mode")
	verifyETMData(s, string(out))

	// Test ETM trace decoding and sample synthesis.
	perfInjectData := filepath.Join(s.OutDir(), "system-wide-perf-inject.data")
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=i1000il", "--strip", "-i", perfData, "-o", perfInjectData)
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("Verifying Last Branch samples in system-wide mode")
	// We can't reliably capture the workload command in system-wide mode
	// because ETM buffer holds only a small sample of ETM trace data where
	// background processes can dominate.
	// ETM tracing with strobing makes sample distribution more uniform and
	// should resolve the issue.
	// TODO(b/200183162): Add Strobing mode testing with command verification.
	verifyLastBranchSamples(s, string(out), "")
}

// PerfETM verifies that cs_etm PMU event is supported and we can collect ETM data and
// convert it into the last branch samples. The test verifies per-thread and system-wide
// perf modes.
func PerfETM(ctx context.Context, s *testing.State) {
	// Verify that perf supports ETM.
	verifyETMIsEnabled(ctx, s)

	// Test ETM in the per-thread mode.
	perfETMPerThread(ctx, s)

	// Test ETM in the system-wide mode.
	perfETMSystemWide(ctx, s)
}
