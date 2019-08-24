// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VTSKeymaster,
		Desc:         "Runs the Android VTS module VtsHalKeymasterV3_0Target",
		Contacts:     []string{"edman@chromium.org", "arc-eng-muc@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		// TODO(edmanp): Download only one file for the current architecture.
		Data: []string{
			"VtsHalKeymasterV3_0TargetTest_arm",
			"VtsHalKeymasterV3_0TargetTest_x86",
			"VtsHalKeymasterV3_0TargetTest_x86_64",
		},
		Pre:     arc.Booted(),
		Timeout: 5 * time.Minute,
	})
}

func VTSKeymaster(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Pushing test binary to ARC")

	testExecName, err := vtsTestExecName(ctx, a)
	if err != nil {
		s.Fatal("Error finding test binary name: ", err)
	}

	testExecPath, err := a.PushFileToTmpDir(ctx, s.DataPath(testExecName))
	if err != nil {
		s.Fatal("Failed to push test binary to ARC: ", err)
	}
	defer a.Command(ctx, "rm", testExecPath).Run()

	if err := a.Command(ctx, "chmod", "0500", testExecPath).Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to change test binary permissions: ", err)
	}

	s.Log("Running tests")

	testCases, err := listTests(ctx, a, testExecPath)
	if err != nil {
		s.Fatal("Failed to list test cases: ", err)
	}
	logdir := filepath.Join(s.OutDir(), "gtest")

	for _, testCase := range testCases {
		s.Log("Running ", testCase)

		logfile := filepath.Join(logdir, testCase+".log")
		if err := runCase(ctx, a, testExecPath, testCase, logfile); err != nil {
			s.Errorf("%s failed: %v", testCase, err)
		}
	}
}

// vtsTestExecName returns the test binary name to be used for the current architecture.
func vtsTestExecName(ctx context.Context, a *arc.ARC) (string, error) {
	output, err := a.Command(ctx, "uname", "-m").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", errors.Wrap(err, "failed to determine container architecture")
	}

	arch := strings.TrimSpace(string(output))
	if arch == "armv8l" {
		return "VtsHalKeymasterV3_0TargetTest_arm", nil
	} else if arch == "i686" {
		return "VtsHalKeymasterV3_0TargetTest_x86", nil
	} else if arch == "x86_64" {
		return "VtsHalKeymasterV3_0TargetTest_x86_64", nil
	}

	return "", errors.Errorf("no known test binary for %s architecture", arch)
}

////////////////////////////////////////////////////////////////////////////////
// TODO(crbug.com/946390): Migrate to gtest package once it supports ARC.

func listTests(ctx context.Context, a *arc.ARC, exec string) ([]string, error) {
	output, err := a.Command(ctx, exec, "--gtest_list_tests").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return parseTestList(string(output)), nil
}

// parseTestList parses the output of --gtest_list_tests and returns the
// list of test names. The format of output should be:
//
// TestSuiteA.
//   TestCase1
//   TestCase2
// TestSuiteB.
//   TestCase3
//   TestCase4
//
// etc. The each returned test name is formatted into "TestSuite.TestCase".
func parseTestList(content string) []string {
	var suite string
	var result []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, " ") {
			// Test case name.
			result = append(result, fmt.Sprintf("%s%s", suite, strings.TrimSpace(line)))
		} else {
			// Test suite name. Note: suite contains a trailing period.
			suite = strings.TrimSpace(line)
		}
	}
	return result
}

// runCase executes the specified testcase. Both stdout and stderr will be
// redirected to logfile.
func runCase(ctx context.Context, a *arc.ARC, exec, testcase, logfile string) error {
	// Ensure the log directory exists.
	if err := os.MkdirAll(filepath.Dir(logfile), 0755); err != nil {
		return err
	}

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(logfile)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := a.Command(ctx, exec, "--gtest_filter="+testcase)
	cmd.Stdout = f
	cmd.Stderr = f
	return cmd.Run()
}
