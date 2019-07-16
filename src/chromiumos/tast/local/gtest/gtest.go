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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
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

// Param has configurations to execute gtest.
type Param struct {
	// Logfile is a path to the log file, which will contain stdout and
	// stderr of the gtest execution.
	// Note: Because the gtest log could be long, if this is not specified,
	// the log wouldn't be recorded to the current test log.
	Logfile string

	// Filter filters to run tests. If given, passed to --gtest_filter=.
	// Please see the gtest manual for the specification.
	Filter string

	// ExtraArgs will be passed to the test execution. Note that all
	// --gtest* prefixed commandline flags should be constructed from
	// Param struct internally, so this should not contain them.
	ExtraArgs []string

	// TODO(hidehiko): To migrate from bintest, support UID.
	// TODO(hidehiko): To migrate from arctest, support ARC gtest run.
}

// Run executes the gtest binary exec with the given param, and then returns
// the parsed Report.
// Note that the returned Report may not nil even if test command execution
// returns an error. E.g., if test case in the gtest fails, the command will
// return an error, but the report file should be created. This function
// also handles the case, and returns it.
func Run(ctx context.Context, exec string, param *Param) (*Report, error) {
	args := []string{exec}
	if param != nil && param.Filter != "" {
		args = append(args, "--gtest_filter="+param.Filter)
	}

	// Create report file.
	output, err := createOutput()
	if err != nil {
		return nil, err
	}
	defer os.Remove(output)
	args = append(args, "--gtest_output=xml:"+output)

	if param != nil && param.ExtraArgs != nil {
		for _, a := range param.ExtraArgs {
			if strings.HasPrefix(a, "--gtest") {
				return nil, errors.Errorf("gtest.Param.ExtraArgs should not contain --gtest prefixed flags: %s", a)
			}
		}
		args = append(args, param.ExtraArgs...)
	}

	cmd := testexec.CommandContext(ctx, args[0], args[1:]...)

	// Set up log file.
	if param != nil && param.Logfile != "" {
		if err := os.MkdirAll(filepath.Dir(param.Logfile), 0755); err != nil {
			return nil, errors.Wrap(err, "failed to create log directory")
		}
		f, err := os.Create(param.Logfile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a log file")
		}
		// f needs to be closed after cmd starts.
		defer f.Close()
		cmd.Stdout = f
		cmd.Stderr = f
	}

	retErr := cmd.Run()

	// Parse output file regardless of whether the command succeeded or
	// not. Specifically, if a test case fail, the command reports an
	// error, but the report file should be properly created.
	report, err := ParseReport(output)
	if err != nil {
		if retErr == nil {
			retErr = err
		} else {
			// Just log the parse error if the command execution
			// already fails.
			testing.ContextLog(ctx, "Failed to parse report: ", err)
		}
	}

	return report, retErr
}

func createOutput() (string, error) {
	f, err := ioutil.TempFile("", "gtest_output_*.xml")
	if err != nil {
		return "", errors.Wrap(err, "failed to create an output file")
	}
	defer func() {
		if f != nil {
			os.Remove(f.Name())
		}
	}()

	if err := f.Close(); err != nil {
		return "", errors.Wrap(err, "failed to close the output file")
	}

	abspath, err := filepath.Abs(f.Name())
	if err != nil {
		return "", errors.Wrap(err, "failed to resolve the path to abs")
	}

	f = nil
	return abspath, nil
}
