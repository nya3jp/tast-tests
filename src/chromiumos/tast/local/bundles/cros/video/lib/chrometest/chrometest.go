// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrometest is used to execute compiled Chrome tests.
package chrometest

import (
	"context"
	"io"
	"io/ioutil"
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
	// tempDir is the temporary directory where the writable file exists.
	tempDir string
}

// NewWritableFile creates WritableFile.
// name is the file name that will be put in outDir.
func NewWritableFile(name string) (*WritableFile, error) {
	td, err := ioutil.TempDir("", "tast_chrometest.")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary directory")
	}
	if err := os.Chmod(td, 0777); err != nil {
		return nil, errors.Wrap(err, "failed to chmod temporary directory")
	}

	return &WritableFile{filepath.Join(td, name), td}, nil
}

// Move moves a file that chrome binary test writes, to outDir.
func (w *WritableFile) Move(outDir string) error {
	srcFile, err := os.Open(w.Path)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", w.Path)
	}
	defer srcFile.Close()

	dstPath := filepath.Join(outDir, filepath.Base(w.Path))
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", dstPath)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return errors.Wrapf(err, "failed to copy from %s to %s", w.Path, dstPath)
	}

	if err := os.Remove(w.Path); err != nil {
		return errors.Wrapf(err, "failed to remove %s", w.Path)
	}
	return nil
}

// Close removes the files in the temporal directory.
func (w *WritableFile) Close() error {
	return os.RemoveAll(w.tempDir)
}
