// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bintest is used to execute compiled Chrome tests.
package bintest

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// getFailedTests returns failed test cases reported by Google Test framework.
// gtestDir should be under the outDir arg passed to an earlier call to Run or RunAsync.
func getFailedTests(ctx context.Context, gtestDir string) []string {
	files, err := ioutil.ReadDir(gtestDir)
	if err != nil {
		testing.ContextLog(ctx, "Ignoring error on reading gtest log directory: ", err)
		return nil
	}

	var res []string
	for _, fi := range files {
		jsonPath := filepath.Join(gtestDir, fi.Name())
		f, err := os.Open(jsonPath)
		if err != nil {
			testing.ContextLog(ctx, "Ignoring error on opening gtest log: ", err)
			continue
		}
		defer f.Close()

		ts, err := extractFailedTests(f)
		if err != nil {
			testing.ContextLog(ctx, "Ignoring error on parsing gtest log: ", err)
			continue
		}

		for _, t := range ts {
			res = append(res, fmt.Sprintf("%s/%s", t.SuiteName, t.CaseName))
		}
	}

	return res
}

const (
	username = "chronos" // user used to run test process
	uid      = 1000      // username's UID
)

// Run executes a Chrome binary test at exec with args.
// If the binary test fails, it returns names of failed test cases and an error.
func Run(ctx context.Context, exec string, args []string, outDir string) ([]string, error) {
	cmd, gtestDir, err := RunAsync(ctx, exec, args, outDir)
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return getFailedTests(ctx, gtestDir), err
	}

	return nil, nil
}

// RunAsync prepares env variables and starts the specified chrome binary test asynchronously, and returns
// a command object and Google Test output path in string for getting failed report.
func RunAsync(ctx context.Context, exec string, args []string, outDir string) (*testexec.Cmd, string, error) {
	binaryTestPath := filepath.Join("/usr/local/libexec/chrome-binary-tests", exec)

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("output_%s_%d.txt", exec, time.Now().Unix())))
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	// gtestDir is the directory where Google Test stores JSON results.
	gtestDir := filepath.Join(outDir, "gtest")

	// Create a directory if not exists where JSON files reporting test results will be stored.
	if err := os.Mkdir(gtestDir, 0755); err != nil && !os.IsExist(err) {
		return nil, "", err
	}
	if err := os.Chown(gtestDir, uid, 0); err != nil {
		return nil, "", err
	}

	// We don't use os.Environ() here. Otherwise, binary executed by "chronos" will fail because
	// they cannot access $TMPDIR which is owned by "root".
	env := []string{fmt.Sprintf("GTEST_OUTPUT=json:%s/", gtestDir)}

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-E", "-u", username, binaryTestPath}, args...)...)
	cmd.Env = env
	cmd.Stdout = f
	cmd.Stderr = f
	testing.ContextLogf(ctx, "Executing %s", testexec.ShellEscapeArray(cmd.Args))
	if err := cmd.Start(); err != nil {
		return nil, "", err
	}

	return cmd, gtestDir, nil
}
