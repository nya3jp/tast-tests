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
		// ETM is the optional HW implemented only on Qualcomm SoCs
		HardwareDeps: hwdep.D(hwdep.Platform("trogdor", "herobrine")),
		Attr:         []string{"group:mainline", "informational"},
	})
}

// verifyETMIsEnabled checks that CoreSight/ETM is enabled in the kernel.
func verifyETMIsEnabled(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "perf", "list")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
		// We can't check the event but if cs_etm is missing we will
		// see the failure of perf record command with the event later.
		return
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

// verifyAndSetETMStrobingConfiguration checks that strobing configuration is
// exposed in configfs and modify ETM strobing parameters.
func verifyAndSetETMStrobingConfiguration(ctx context.Context, s *testing.State) {
	s.Log("-------------------------------------")
	s.Log("Verifying and setting up ETM strobing")
	for _, param := range []struct {
		name  string
		path  string
		value string
	}{
		{
			name:  "period",
			path:  "/sys/kernel/config/cs-syscfg/features/strobing/params/period/value",
			value: "0x800",
		},
		{
			name:  "window",
			path:  "/sys/kernel/config/cs-syscfg/features/strobing/params/window/value",
			value: "0x400",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Read the default parameter.
			defaultParam, err := ioutil.ReadFile(param.path)
			if err != nil {
				s.Errorf("Failed to read %v, error %v", param.path, err)
				return
			}
			// Set new strobing parameters.
			if err = ioutil.WriteFile(param.path, []byte(param.value), 0644); err != nil {
				s.Errorf("Write to %q failed: %v", param.path, err)
				return
			}
			// Verify the new strobing settings.
			readBackParam, err := ioutil.ReadFile(param.path)
			if err != nil {
				s.Errorf("Failed to read %v, error %v", param.path, err)
				return
			}
			readBackParamStr := strings.TrimSpace(string(readBackParam))
			s.Logf("New strobing %v: %v", param.name, readBackParamStr)
			if strings.Compare(readBackParamStr, param.value) != 0 {
				s.Errorf("Failed to update strobing parameter %v. Was %v, modified to %v, read back %v",
					param.path, strings.TrimSpace(string(defaultParam)), param.value, readBackParamStr)
			}
		})
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
	s.Fatal("Couldn't find AUX buffer in perf report")
}

// verifyLastBranchSamples verifies that the report contains a last branch sample.
// We can verify either "dso" or "tracedCommand" but not both at the same time.
// If "tracedCommand" is non empty verify that the command has branch records.
// If "dso" is non empty verify that there are records belonging to this dso.
func verifyLastBranchSamples(s *testing.State, report, tracedCommand, dso string) {
	if tracedCommand != "" && dso != "" {
		s.Fatal("Can't verify \"tracedCommand\" and \"dso\" at the same time. Split it into two calls")
	}
	sampleRecordRegexp := regexp.MustCompile("PERF_RECORD_SAMPLE")
	branchStackSizeRegexp := regexp.MustCompile(`branch stack: nr:(\d+)`)
	threadRegexp := regexp.MustCompile(`thread: (\S+):\d+`)
	dsoRegexp := regexp.MustCompile(`dso: (\S+)`)
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

		if tracedCommand != "" {
			threadMatch := threadRegexp.FindStringSubmatch(record)
			if threadMatch != nil && tracedCommand == threadMatch[1] {
				s.Logf("Found a sample with %q, stack size %v", tracedCommand, size)
				return
			}
			// Record is either invalid or belongs to a different command.
			continue
		}
		if dso != "" {
			dsoMatch := dsoRegexp.FindStringSubmatch(record)
			if dsoMatch != nil && dso == dsoMatch[1] {
				s.Logf("Found a sample from dso %q, stack size %v", dso, size)
				return
			}
			// Record is either invalid or belongs to a different dso.
			continue
		}
		// We are ok with any last branch record if neither tracedCommand or dso is passed.
		s.Log("Found a sample with stack size ", size)
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
		// Return early since we don't have a profile with last branch samples.
		return
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
		return
	}
	s.Log("Verifying Last Branch samples in per-thread mode")
	verifyLastBranchSamples(s, string(out), tracedCommand, "")
}

// perfETMSystemWide records ETM trace in system-wide mode and verifies the raw dump.
func perfETMSystemWide(ctx context.Context, s *testing.State) {
	perfData := filepath.Join(s.OutDir(), "system-wide-perf.data")
	perfCommand := []string{"record", "-e", "cs_etm/autofdo/uk", "-N", "-o", perfData, "-a", "--"}
	tracedCommand := []string{"top", "-b", "-n10", "-d0.1"}
	fullPerfCommand := append(perfCommand, tracedCommand...)

	// Test ETM profile collection.
	cmd := testexec.CommandContext(ctx, "perf", fullPerfCommand...)
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
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=i1024il", "--strip", "-i", perfData, "-o", perfInjectData)
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
		// Return early since we don't have a profile with last branch samples.
		return
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Errorf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
		return
	}
	s.Log("Verifying Last Branch samples in system-wide mode")
	// Verify samples from the traced command.
	verifyLastBranchSamples(s, string(out), tracedCommand[0], "")
	// Verify samples from the kernel dso.
	verifyLastBranchSamples(s, string(out), "", "/proc/kcore")
}

// PerfETM verifies that cs_etm PMU event is supported and we can collect ETM data and
// convert it into the last branch samples. The test verifies per-thread and system-wide
// perf modes.
func PerfETM(ctx context.Context, s *testing.State) {
	// Verify that perf supports ETM.
	verifyETMIsEnabled(ctx, s)

	// Verify and set up ETM strobing configuration.
	verifyAndSetETMStrobingConfiguration(ctx, s)

	// Test ETM in the per-thread mode.
	perfETMPerThread(ctx, s)

	// Test ETM in the system-wide mode.
	perfETMSystemWide(ctx, s)
}
