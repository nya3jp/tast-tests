// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrostiniCPUPerf,
		Desc: "Tests Crostini CPU performance",
		// TODO(cylee): A presubmit check enforce "informational". Confirm if we should remove the checking.
		Attr: []string{"informational", "group:crosbolt", "crosbolt_nightly"},
		// Data:         dataFiles,
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

// toTimeUnit returns time.Duration in unit |unit| as float64 numbers.
func toTimeUnit(unit time.Duration, ts ...time.Duration) (out []float64) {
	for _, t := range ts {
		out = append(out, float64(t)/float64(unit))
	}
	return out
}

func CrostiniCPUPerf(ctx context.Context, s *testing.State) {
	// TODO(cylee): Consolidate container creation logic in a util function since it appears in multiple files.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	// TODO(cylee): Consolidate similar util function in other test files.
	// Prepare error log file.
	errFile, err := os.Create(filepath.Join(s.OutDir(), "error_log.txt"))
	if err != nil {
		s.Fatal("Failed to create error log: ", err)
	}
	defer errFile.Close()
	writeError := func(title string, content []byte) {
		const logTemplate = "========== START %s ==========\n%s\n========== END ==========\n"
		if _, err := fmt.Fprintf(errFile, logTemplate, title, content); err != nil {
			s.Log("Failed to write error to log file: ", err)
		}
	}
	runCmd := func(cmd *testexec.Cmd) (out []byte, err error) {
		// lmbench somehow outputs to stderr instead of stdout, so we need combined output here.
		out, err = cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		cmdString := strings.Join(append(cmd.Cmd.Env, cmd.Cmd.Args...), " ")

		// Dump stderr.
		if err := cmd.DumpLog(ctx); err != nil {
			s.Logf("Failed to dump log for cmd %q: %v", cmdString, err)
		}

		// Output complete stdout to a log file.
		writeError(cmdString, out)

		// Only append the first and last line of the output to the error.
		out = bytes.TrimSpace(out)
		var errSnippet string
		if idx := bytes.IndexAny(out, "\r\n"); idx != -1 {
			lastIdx := bytes.LastIndexAny(out, "\r\n")
			errSnippet = fmt.Sprintf("%s ... %s", out[:idx], out[lastIdx+1:])
		} else {
			errSnippet = string(out)
		}
		return []byte{}, errors.Wrap(err, errSnippet)
	}

	// Install needed packages.
	// TODO(cylee): remove the installation code once CL:1401301 is submitted.
	s.Log("Updating apt source list")
	addNonFreeRepoCmdArgs := []string{
		"sudo", "sed", "-E", "-i", "s|^(deb.*main)$|\\1 non-free|g", "/etc/apt/sources.list"}
	if _, err := runCmd(cont.Command(ctx, addNonFreeRepoCmdArgs...)); err != nil {
		s.Fatal("Failed to modify apt source.list: ", err)
	}
	s.Log("apt-get update")
	if _, err := runCmd(cont.Command(ctx, "sudo", "apt-get", "update")); err != nil {
		s.Fatal("Failed to run apt-get update: ", err)
	}
	packages := []string{
		"lmbench",
	}
	s.Log("Installing ", packages)
	installCmdArgs := append([]string{"sudo", "apt-get", "-y", "install"}, packages...)
	if _, err := runCmd(cont.Command(ctx, installCmdArgs...)); err != nil {
		s.Fatalf("Failed to install needed packages %v: %v", packages, err)
	}

	// Latest lmbench defaults to install individual microbenchamrks in /usr/lib/lmbench/bin/<arch dependent folder>
	// (e.g., /usr/lib/lmbench/bin/x86_64-linux-gnu). So needs to find the exact path.
	out, err := runCmd(cont.Command(ctx, "find", "/usr/lib/lmbench", "-name", "lat_syscall"))
	if err != nil {
		s.Fatal("Failed to find syscall benchmark binary in container: ", err)
	}
	guestSyscallBenchBinary := strings.TrimSpace(string(out))
	s.Log("Found syscall benchamrk installed in container: ", guestSyscallBenchBinary)

	// Perf output
	perfValues := perf.Values{}
	defer perfValues.Save(s.OutDir())

	// Output parser. Sample output: "Simple write: 0.2412 microseconds".
	// It's always in microseconds for lat_syscall.
	var parseSyscallBenchOutput = func(out string) (t time.Duration, err error) {
		samplePattern := regexp.MustCompile(`.*: (\d*\.?\d+) microseconds`)
		matched := samplePattern.FindStringSubmatch(strings.TrimSpace(out))
		if matched != nil {
			t, err := strconv.ParseFloat(matched[1], 64)
			if err != nil {
				return 0.0, errors.Wrapf(err, "failed to parse time %q in lat_syscall output", matched[1])
			}
			return time.Duration(t * float64(time.Microsecond)), nil
		}
		return 0.0, errors.Errorf("unable to match time from %q", out)
	}

	// Measure syscall time.
	var measureSyscallTime = func(args ...string) error {
		options := []string{
			"-N", "10", // repetition times.
		}
		allArgs := append(options, args...)

		// Current version of lmbench on CrOS installs individual benchmarks in /usr/local/bin so
		// can be called directly.
		out, err := runCmd(testexec.CommandContext(ctx, "lat_syscall", allArgs...))
		if err != nil {
			return errors.Wrap(err, "failed to run lat_syscall on host")
		}
		hostTime, err := parseSyscallBenchOutput(string(out))
		if err != nil {
			return errors.Wrap(err, "failed to parse lat_syscall output on host")
		}

		// Guest binary is in /usr/lib/lmbench/...
		guestCommandArgs := append([]string{guestSyscallBenchBinary}, allArgs...)
		out, err = runCmd(cont.Command(ctx, guestCommandArgs...))
		if err != nil {
			return errors.Wrap(err, "failed to run lat_syscall on guest")
		}
		guestTime, err := parseSyscallBenchOutput(string(out))
		if err != nil {
			return errors.Wrap(err, "failed to parse lat_syscall output on guest")
		}

		// Output.
		ratio := float64(guestTime) / float64(hostTime)
		s.Logf("syscall %v: host %v, guest %v, guest/host ratio %.2f\n", args[0], hostTime, guestTime, ratio)

		var metricName = func(subName string) string {
			sysCallName := args[0]
			// The name "null" actually runs getpid() underneath.
			if sysCallName == "null" {
				sysCallName = "getpid"
			}
			return fmt.Sprintf("syscall_%s_%s", sysCallName, subName)
		}

		perfValues.Set(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("host"),
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			toTimeUnit(time.Microsecond, hostTime)...)
		perfValues.Set(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("guest"),
				Unit:      "microseconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			toTimeUnit(time.Microsecond, guestTime)...)
		perfValues.Set(
			perf.Metric{
				Name:      "crostini_cpu",
				Variant:   metricName("ratio"),
				Unit:      "percentage",
				Direction: perf.SmallerIsBetter,
				Multiple:  false,
			},
			ratio)
		return nil
	}

	// lat_syscall reads /dev/zero and writes to /dev/null. "null" calls getpid().
	for _, syscall := range []string{"null", "read", "write"} {
		if err := measureSyscallTime(syscall); err != nil {
			s.Errorf("Failed to measure syscall time for command %v: %v", syscall, err)
		}
	}
	// The three commands operate on a file.
	for _, syscall := range []string{"stat", "fstat", "open"} {
		if err := measureSyscallTime(syscall, "/bin/ls"); err != nil {
			s.Errorf("Failed to measure syscall time for command %v: %v", syscall, err)
		}
	}
}
