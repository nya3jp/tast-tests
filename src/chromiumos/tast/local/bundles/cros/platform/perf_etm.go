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
	"chromiumos/tast/errors"
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

// testETMEventAvailable checks that CoreSight/ETM is enabled on the device.
func testETMEventAvailable(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "perf", "list")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	perfListFile := filepath.Join(s.OutDir(), "perf-list.txt")
	if err = ioutil.WriteFile(perfListFile, out, 0644); err != nil {
		s.Fatalf("Write to %q failed: %v", perfListFile, err)
	}
	// Make sure CoreSight is listed in the Kernel PMU events.
	if !strings.Contains(string(out), "cs_etm") {
		s.Fatal("CoreSight/ETM is not enabled on the device")
	}
}

// testSettingETMStrobingConfiguration checks that strobing configuration is
// exposed in configfs and modifies ETM strobing parameters.
func testSettingETMStrobingConfiguration(ctx context.Context, s *testing.State) {
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
				s.Fatalf("Failed to read %v, error %v", param.path, err)
			}
			// Set new strobing parameters.
			if err = ioutil.WriteFile(param.path, []byte(param.value), 0644); err != nil {
				s.Fatalf("Write to %q failed: %v", param.path, err)
			}
			// Verify the new strobing settings.
			readBackParam, err := ioutil.ReadFile(param.path)
			if err != nil {
				s.Fatalf("Failed to read %v, error %v", param.path, err)
			}
			readBackParamStr := strings.TrimSpace(string(readBackParam))
			if strings.Compare(readBackParamStr, param.value) != 0 {
				s.Fatalf("Failed to update strobing parameter %v. Was %v, modified to %v, read back %v",
					param.path, strings.TrimSpace(string(defaultParam)), param.value, readBackParamStr)
			}
		})
	}
}

// verifyETMData verifies that the report contains an AUX record with ETM data.
func verifyETMData(report string) error {
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
		return nil
	}
	return errors.New("couldn't find AUX buffer in perf report")
}

// verifyLastBranchSamples verifies that the report contains a last branch sample.
// We can verify either "dso" or "tracedCommand" but not both at the same time.
// If "tracedCommand" is non empty verify that the command has branch records.
// If "dso" is non empty verify that there are records belonging to this dso.
func verifyLastBranchSamples(report, tracedCommand, dso string) error {
	if tracedCommand != "" && dso != "" {
		return errors.New("can't verify \"tracedCommand\" and \"dso\" at the same time. Split it into two calls")
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
				// Found a branch sample from the traced command.
				return nil
			}
			// Record is either invalid or belongs to a different command.
			continue
		}
		if dso != "" {
			dsoMatch := dsoRegexp.FindStringSubmatch(record)
			if dsoMatch != nil && dso == dsoMatch[1] {
				// Found a branch sample from the dso.
				return nil
			}
			// Record is either invalid or belongs to a different dso.
			continue
		}
		// We are ok with any last branch record if neither tracedCommand or dso is passed.
		return nil
	}
	return errors.Errorf("couldn't find a valid Last Branch sample. Total number of samples: %d", numberOfRecords)
}

// testPerfETMPerThread records ETM trace in per-thread mode and verifies the raw dump.
func testPerfETMPerThread(ctx context.Context, s *testing.State) {
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
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("Verifying ETM data in the per-thread mode")
	if err = verifyETMData(string(out)); err != nil {
		s.Fatal("ETM data verification failed in the per-thread mode: ", err)
	}

	// Test ETM trace decoding and sample synthesis.
	perfInjectData := filepath.Join(s.OutDir(), "per-thread-perf-inject.data")
	// --strip reduces the output size.
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=i1000il", "-i", perfData, "-o", perfInjectData, "--strip")
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("Verifying Last Branch samples in per-thread mode")
	if err = verifyLastBranchSamples(string(out), tracedCommand, ""); err != nil {
		s.Errorf("Last branch sample verification failed for %q command: %v", tracedCommand, err)
	}
}

// testPerfETMSystemWide records ETM trace in system-wide mode and verifies the raw dump.
func testPerfETMSystemWide(ctx context.Context, s *testing.State) {
	perfData := filepath.Join(s.OutDir(), "system-wide-perf.data")
	perfCommand := []string{"record", "-e", "cs_etm/autofdo/uk", "-N", "-o", perfData, "-a", "--"}
	tracedCommand := []string{"top", "-b", "-n10", "-d0.1"}
	fullPerfCommand := append(perfCommand, tracedCommand...)
	kernelDSO := "/proc/kcore"

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
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("Verifying ETM data in the system-wide mode")
	if err = verifyETMData(string(out)); err != nil {
		s.Fatal("ETM data verification failed in the system-wide mode: ", err)
	}

	// Test ETM trace decoding and sample synthesis.
	perfInjectData := filepath.Join(s.OutDir(), "system-wide-perf-inject.data")
	cmd = testexec.CommandContext(ctx, "perf", "inject", "--itrace=Zi1024il", "--strip", "-i", perfData, "-o", perfInjectData)
	err = cmd.Run(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}

	// Test ETM data in the profile with synthesized branch samples.
	cmd = testexec.CommandContext(ctx, "perf", "report", "-D", "-i", perfInjectData)
	out, err = cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("%s failed: %v", shutil.EscapeSlice(cmd.Args), err)
	}
	s.Log("Verifying Last Branch samples in system-wide mode")
	// Verify samples from the traced command.
	if err = verifyLastBranchSamples(string(out), tracedCommand[0], ""); err != nil {
		// TODO(b/225407997): Replace with Errorf when fixed.
		s.Logf("Failed but forgiven. Last branch sample verification failed for %q command: %v", tracedCommand[0], err)
	}
	// Verify samples from the kernel dso.
	if err = verifyLastBranchSamples(string(out), "", kernelDSO); err != nil {
		s.Errorf("Last branch sample verification failed for %q dso: %v", kernelDSO, err)
	}
}

// PerfETM verifies that cs_etm PMU event is supported and we can collect ETM data and
// convert it into the last branch samples. The test verifies ETM strobing and
// ETM tracing in per-thread and system-wide perf modes.
func PerfETM(ctx context.Context, s *testing.State) {
	// Test that ETM is enabled.
	s.Run(ctx, "ETM Event", testETMEventAvailable)

	// Test setting up ETM strobing configuration.
	s.Run(ctx, "ETM Strobing", testSettingETMStrobingConfiguration)

	// Test ETM tracing in the per-thread mode.
	s.Run(ctx, "ETM per-thread trace", testPerfETMPerThread)

	// Test ETM tracing in the system-wide mode.
	s.Run(ctx, "ETM system-wide trace", testPerfETMSystemWide)
}
