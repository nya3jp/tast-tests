// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"chromiumos/tast/common/testexec"
)

// IgtTest is used to describe the config used to run each test.
type IgtTest struct {
	Exe string // The test executable name.
}

// igtResultSummary is a summary of results from an igt test log.
type igtResultSummary struct {
	passed  int // number of passed subtests
	failed  int // number of failed subtests
	skipped int // number of skipped subtests
}

var igtSubtestResultRegex = regexp.MustCompile("^Subtest (.*): ([A-Z]+)")

// IgtExecuteTests executes the IGT binary of the test exe.
func IgtExecuteTests(ctx context.Context, testExe string, f *os.File) (bool, *exec.ExitError, error) {
	exePath := filepath.Join("/usr/local/libexec/igt-gpu-tools", testExe)
	cmd := testexec.CommandContext(ctx, exePath)
	cmd.Stdout = f
	cmd.Stderr = f
	err := cmd.Run()
	exitErr, isExitErr := err.(*exec.ExitError)

	// Reset the file to the beginning so the log can be read out again.
	f.Seek(0, 0)

	return isExitErr, exitErr, err
}

func igtSummarizeLog(f *os.File) (r igtResultSummary, failedSubtests []string) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := igtSubtestResultRegex.FindStringSubmatch(scanner.Text()); m != nil {
			subtestName := m[1]
			result := m[2]
			switch result {
			case "SKIP":
				r.skipped++
			case "FAIL":
				r.failed++
				failedSubtests = append(failedSubtests, subtestName)
			case "SUCCESS":
				r.passed++
			}
		}
	}
	return r, failedSubtests
}

// IgtProcessResults reads the results of the test output and outputs a summary of the full test results.
func IgtProcessResults(testExe string, file *os.File, isExitErr bool, exitErr *exec.ExitError, err error) (bool, string) {
	results, failedSubtests := igtSummarizeLog(file)
	summary := fmt.Sprintf("Ran %d subtests with %d failures and %d skipped",
		results.passed+results.failed, results.failed, results.skipped)

	isError := false
	outputLog := ""

	if results.passed+results.failed+results.skipped == 0 {
		// TODO(markyacoub): Many tests have igt_require_intel(), which automatically skips
		// everything on other platforms. Mark the test as PASS for now until there are no more
		// platform specific dependencies
		outputLog = "Entire test was skipped - No subtests were run\n"
		// In the case of running multiple subtests which all happen to be skipped, igt_exitcode is 0,
		// but the final exit code will be 77.
	} else if results.passed+results.failed == 0 && isExitErr && exitErr.ExitCode() == 77 {
		outputLog = "____________________________________________________\n"
		outputLog += fmt.Sprintf("ALL %d subtests were SKIPPED: %s\n", results.skipped, err.Error())
		outputLog += "----------------------------------------------------"
	} else if len(failedSubtests) > 0 {
		outputLog = fmt.Sprintf("FAIL: Test:%s - Pass:%d Fail:%d - FailedSubtests:%s - Summary:%s\n",
			testExe, results.passed, results.failed, failedSubtests, summary)
		isError = true
	} else {
		outputLog = fmt.Sprintf("%s\n", summary)
	}

	return isError, outputLog
}
