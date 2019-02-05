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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/security/filesetup"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// getResultDirPath returns a path of a directory where Google Test will store JSON files.
func getResultDirPath(outDir string) string {
	return filepath.Join(outDir, "results")
}

// Run executes a Chrome binary test at exec with args.
// It returns an error if the binary test fails.
func Run(ctx context.Context, exec string, args []string, outDir string) error {
	cmd, err := RunAsync(ctx, exec, args, outDir)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

// RunAsync starts the specified chrome binary test asynchronously and returns
// a command object.
func RunAsync(ctx context.Context, exec string, args []string, outDir string) (*testexec.Cmd, error) {
	binaryTestPath := filepath.Join("/usr/local/libexec/chrome-binary-tests", exec)

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("output_%s_%d.txt", exec, time.Now().Unix())))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const username = "chronos"
	// Create a directory where JSON files reporting test results will be stored.
	// Since this function can be called multiple times, the directly may exist already.
	resultDir := getResultDirPath(outDir)
	if _, err := os.Stat(resultDir); os.IsNotExist(err) {
		uid := filesetup.GetUID(username)
		filesetup.CreateDir(resultDir, uid, 0755)
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

// GetFailedCases returns a string containing failed test cases reported by Google Test framework.
func GetFailedCases(outDir string) ([]string, error) {
	resultDir := getResultDirPath(outDir)

	files, err := ioutil.ReadDir(resultDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read %s", resultDir)
	}

	res := []string{}
	for _, f := range files {
		jsonPath := filepath.Join(resultDir, f.Name())
		s, err := ioutil.ReadFile(jsonPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %s", jsonPath)
		}

		fs, err := extractFailedCases(s)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", jsonPath)
		}

		for _, f := range fs {
			res = append(res, fmt.Sprintf("%s/%s\n", f.SuiteName, f.CaseName))
		}
	}

	return res, nil
}
