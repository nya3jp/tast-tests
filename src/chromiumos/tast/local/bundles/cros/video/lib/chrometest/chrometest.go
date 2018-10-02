// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrometest is used to execute compiled Chrome tests.
package chrometest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Run executes a chrome binary test, execFileName, with args. This returns error if
// the chrome binary test fails.
func Run(ctx context.Context, outDir, execFileName string, args []string) error {
	const binaryTestDir = "/usr/local/libexec/chrome-binary-tests/"
	binaryTestPath := filepath.Join(binaryTestDir, execFileName)

	// Create the output file that the test log is dumped on failure.
	f, err := os.Create(filepath.Join(outDir, "output_"+execFileName+"_"+strconv.FormatInt(time.Now().Unix(), 10)))
	if err != nil {
		errors.Wrap(err, "failed to create output file")
	}
	defer f.Close()

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-u", "chronos", binaryTestPath}, args...)...)
	cmd.Env = append(os.Environ(),
		"CHROME_DEVEL_SANDBOX=/opt/google/chrome/chrome-sandbox",
	)
	cmd.Stdout = f
	cmd.Stderr = f

	testing.ContextLogf(ctx, "Executing %s %s", execFileName, testexec.ShellEscapeArray(args))
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "%s failed", binaryTestPath)
	}
	return nil
}

// WritableFile is struct to create a file that a chrome binary test can write to.
type WritableFile struct {
	// Path is the file path that a chrome test actually writes to.
	Path string
	// outDir is the directory where the writable file exists in the end.
	outDir string
}

// NewWritableFile creates WritableFile.
// name is the file name that will be put in outDir.
func NewWritableFile(outDir, name string) *WritableFile {
	return &WritableFile{filepath.Join("/tmp", name), outDir}
}

// Move moves a file that chrome binary test writes, to outDir.
func (w *WritableFile) Move() error {
	return copyToOutDir(w.outDir, w.Path)
}

// copyToOutDir copies to file in srcPath to outDir.
func copyToOutDir(outDir, srcPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", srcPath)
	}
	defer srcFile.Close()

	dstPath := filepath.Join(outDir, filepath.Base(srcPath))
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", dstPath)
	}
	defer dstFile.Close()

	if _, err := io.Copy(srcFile, dstFile); err != nil {
		return errors.Wrapf(err, "failed to copy from %s to %s", srcPath, dstPath)
	}

	if err := srcFile.Close(); err != nil {
		return errors.Wrapf(err, "failed to close %s", srcPath)
	}

	if err := os.Remove(srcPath); err != nil {
		return errors.Wrapf(err, "failed to remove %s", srcPath)
	}
	return nil
}
