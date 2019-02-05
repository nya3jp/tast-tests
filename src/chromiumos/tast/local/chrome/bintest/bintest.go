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
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// resultSubDir is the directory under each test's output directory where Google Test stores JSON results.
const resultSubDir = "results"

// Run executes a Chrome binary test at exec with args.
// If the binary test fails, it returns names of failed test cases and an error.
func Run(ctx context.Context, outDir, exec string, args []string) ([]string, error) {
	cmd, err := RunAsync(ctx, outDir, exec, args)
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return GetFailedTests(ctx, outDir), err
	}

	return nil, nil
}

// RunAsync starts the specified chrome binary test asynchronously and returns
// a command object.
// If the command object's Wait returns an error eventually, GetFailedTests
// can be used to get a list of failed test cases.
func RunAsync(ctx context.Context, outDir, exec string, args []string) (*testexec.Cmd, error) {
	binaryTestPath := filepath.Join("/usr/local/libexec/chrome-binary-tests", exec)

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("output_%s_%d.txt", exec, time.Now().Unix())))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Create a directory where JSON files reporting test results will be stored.
	resultDir := filepath.Join(outDir, resultSubDir)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create %s", resultDir)
	}
	// Change the owner of the created directory.
	const username = "chronos"
	u, err := user.Lookup(username)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to look up user %s", username)
	}
	uid, err := strconv.ParseInt(u.Uid, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse UID %s for user %s", u.Uid, username)
	}
	if err := os.Chown(resultDir, int(uid), 0); err != nil {
		return nil, errors.Wrapf(err, "failed to chown %s to %d", resultDir, uid)
	}

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-E", "-u", username, binaryTestPath}, args...)...)
	cmd.Env = []string{fmt.Sprintf("GTEST_OUTPUT=json:%s/", resultDir)}
	cmd.Stdout = f
	cmd.Stderr = f
	testing.ContextLogf(ctx, "Executing %s", testexec.ShellEscapeArray(cmd.Args))
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// GetFailedTests returns failed test cases reported by Google Test framework.
func GetFailedTests(ctx context.Context, outDir string) []string {
	resultDir := filepath.Join(outDir, resultSubDir)

	files, err := ioutil.ReadDir(resultDir)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to read %s: %v", resultDir, err)
		return nil
	}

	var res []string
	for _, f := range files {
		jsonPath := filepath.Join(resultDir, f.Name())
		r, err := os.Open(jsonPath)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to read %s: %v", jsonPath, err)
			return nil
		}

		fs, err := extractFailedTests(r)
		if err != nil {
			testing.ContextLogf(ctx, "Failed to parse %s: %v", jsonPath, err)
			return nil
		}

		for _, f := range fs {
			res = append(res, fmt.Sprintf("%s/%s", f.SuiteName, f.CaseName))
		}
	}

	return res
}
