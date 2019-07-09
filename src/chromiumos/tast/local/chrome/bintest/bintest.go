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

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
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
		report, err := gtest.ParseReport(filepath.Join(gtestDir, fi.Name()))
		if err != nil {
			testing.ContextLog(ctx, "Ignoring error on parsing gtest log: ", err)
			continue
		}
		res = append(res, report.FailedTestNames()...)
	}

	return res
}

const (
	username = "chronos" // user used to run test process
)

// Run executes a Chrome binary test at exec with args.
// If the binary test fails, it returns names of failed test cases and an error.
func Run(ctx context.Context, exec string, args []string, outDir string) ([]string, error) {
	// gtestDir is the directory where Google Test stores XML results.
	gtestDir := filepath.Join(outDir, "gtest")

	// Create a directory where XML files reporting test results will be stored.
	if err := os.MkdirAll(gtestDir, 0755); err != nil {
		return nil, err
	}
	if err := os.Chown(gtestDir, int(sysutil.ChronosUID), 0); err != nil {
		return nil, err
	}

	// We don't use os.Environ() here. Otherwise, binary executed by "chronos" will fail because
	// they cannot access $TMPDIR which is owned by "root".
	env := []string{fmt.Sprintf("GTEST_OUTPUT=xml:%s/", gtestDir)}
	cmd, err := RunAsync(ctx, exec, args, env, outDir)
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return getFailedTests(ctx, gtestDir), err
	}

	return nil, nil
}

// RunAsync starts the specified chrome binary test asynchronously and returns
// a command object.
func RunAsync(ctx context.Context, exec string, args, env []string, outDir string) (*testexec.Cmd, error) {
	binaryTestPath := filepath.Join("/usr/local/libexec/chrome-binary-tests", exec)

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("output_%s_%d.txt", exec, time.Now().Unix())))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-E", "-u", username, binaryTestPath}, args...)...)
	cmd.Env = env
	cmd.Stdout = f
	cmd.Stderr = f
	testing.ContextLogf(ctx, "Executing %s", shutil.EscapeSlice(cmd.Args))
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}
