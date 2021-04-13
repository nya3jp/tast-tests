// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
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
		Func: PerfEtm,
		Desc: "Verify ETM functionality with the perf tool",
		Contacts: []string{
			"denik@chromium.org",
			"c-compiler-chrome@google.com",
		},
		SoftwareDeps: []string{"arm"},
		HardwareDeps: hwdep.D(hwdep.Platform("trogdor")),
		Attr:         []string{"group:mainline", "informational"},
	})
}

// verifyEtmIsEnabled checks that CoreSight/ETM is enabled in the kernel.
func verifyEtmIsEnabled(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "perf", "list")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}
	if !strings.Contains(string(out), "cs_etm") {
		// Make sure CoreSight is listed in the Kernel PMU events.
		s.Fatal("CoreSight/ETM is not enabled on the device")
	}
}

// verifyEtmData verifies that the report contains an AUX record with ETM data.
func verifyEtmData(s *testing.State, report string) {
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
	for _, record := range records {
		sampleMatch := sampleRecordRegexp.FindString(record)
		if sampleMatch == "" {
			continue
		}
		s.Log("Found Last Branch sample")
		bsSizeMatch := branchStackSizeRegexp.FindStringSubmatch(record)
		if bsSizeMatch == nil {
			continue
		}
		var size int
		var err error
		if size, err = strconv.Atoi(bsSizeMatch[1]); err != nil || size == 0 {
			continue
		}
		s.Log("with non-zero size")
		threadMatch := threadRegexp.FindStringSubmatch(record)
		if threadMatch == nil || threadMatch[1] != tracedCommand {
			continue
		}
		s.Logf("Found a sample with %q, stack size %v", tracedCommand, size)
		return
	}
	s.Error("Couldn't find a valid Last Branch sample")
}

// perfEtmPerThread records ETM trace in per-thread mode and verifies the raw dump.
func perfEtmPerThread(ctx context.Context, s *testing.State) {
	tracedCommand := "ls"
	perfData := filepath.Join(s.OutDir(), "per-thread-perf.data")

	// Test ETM profile collection.
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cs_etm/@tmc_etr0/", "-o", perfData, "--per-thread", tracedCommand)
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the raw profile dump.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfData)
	var out []byte
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}
	verifyEtmData(s, string(out))

	// Test ETM trace decoding and sample synthesis.
	perfInjectData := filepath.Join(s.OutDir(), "per-thread-perf-inject.data")
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=i1000il", "-i", perfData, "-o", perfInjectData)
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}
	verifyLastBranchSamples(s, string(out), tracedCommand)
}

// perfEtmSystemWide records ETM trace in system-wide mode and verifies the raw dump.
func perfEtmSystemWide(ctx context.Context, s *testing.State) {
	tracedCommand := "ls"
	perfData := filepath.Join(s.OutDir(), "system-wide-perf.data")

	// Test ETM profile collection.
	cmd := testexec.CommandContext(ctx, "perf", "record", "-e", "cs_etm/@tmc_etr0/uk", "-o", perfData, "-a", tracedCommand)
	err := cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the raw profile dump.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfData)
	var out []byte
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}
	verifyEtmData(s, string(out))

	// Test ETM trace decoding and sample synthesis.
	perfInjectData := filepath.Join(s.OutDir(), "system-wide-perf-inject.data")
	// TODO(b/163172096): Change to period 1000 to improve the quality of the profile when perf inject hang is fixed.
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=i100000il", "-i", perfData, "-o", perfInjectData)
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%q failed, %v", shutil.EscapeSlice(cmd.Args), err)
	}
	// TODO(b/168574788, b/163171598): Enable Last branch sample
	// verification when all bugs in system-wide mode are resolved.
	if false {
		verifyLastBranchSamples(s, string(out), tracedCommand)
	}
}

// PerfEtm verifies that cs_etm PMU event is supported and we can collect ETM data and
// convert it into the last branch samples. The test verifies per-thread and system-wide
// perf modes.
func PerfEtm(ctx context.Context, s *testing.State) {
	// Verify that perf supports ETM.
	verifyEtmIsEnabled(ctx, s)

	// Test ETM in the per-thread mode.
	perfEtmPerThread(ctx, s)

	// Test ETM in the system-wide mode.
	perfEtmSystemWide(ctx, s)
}
