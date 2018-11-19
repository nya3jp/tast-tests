// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bintest is used to execute compiled Chrome tests.
package bintest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Run executes a Chrome binary test at exec with args.
// It returns an error if the binary test fails.
func Run(ctx context.Context, exec string, args []string, outDir string) error {
	binaryTestPath := filepath.Join("/usr/local/libexec/chrome-binary-tests", exec)

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("output_%s_%d.txt", exec, time.Now().Unix())))
	if err != nil {
		return err
	}
	defer f.Close()

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-u", "chronos", binaryTestPath}, args...)...)
	cmd.Env = append(os.Environ(), "CHROME_DEVEL_SANDBOX=/opt/google/chrome/chrome-sandbox")
	cmd.Stdout = f
	cmd.Stderr = f

	testing.ContextLogf(ctx, "Executing %s", testexec.ShellEscapeArray(cmd.Args))
	return cmd.Run()
}
