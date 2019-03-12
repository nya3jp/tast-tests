// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gtest provides utilities to run gtest binary on Tast.
// TODO(hidehiko): Merge chromiumos/tast/local/chrome/bintest package into
// this.
package gtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/testexec"
)

// ListTests returns a list of tests in the gtest executable.
func ListTests(ctx context.Context, exec string) ([]string, error) {
	output, err := testexec.CommandContext(ctx, exec, "--gtest_list_tests").Output(testexec.DumpLogOnError)
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

// RunCase executes the specified testcase. Both stdout and stderr will be
// redirected to logfile.
func RunCase(ctx context.Context, exec, testcase, logfile string) error {
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

	cmd := testexec.CommandContext(ctx, exec, "--gtest_filter="+testcase)
	cmd.Stdout = f
	cmd.Stderr = f
	return cmd.Run()
}
