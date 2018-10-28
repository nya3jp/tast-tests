// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrometest is used to execute compiled Chrome tests.
package chrometest

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
	f, err := os.Create(filepath.Join(outDir, fmt.Sprintf("output_%s_%d.txt", execFileName, time.Now().Unix())))
	if err != nil {
		return err
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

// WritableFile holds output from a Chrome binary test.
type WritableFile string

const tmpSuffix = ".tast_chrometest."

// NewWritableFile creates WritableFile.
// name is the file name that will be put in outDir.
func NewWritableFile(name string) (WritableFile, error) {
	tf, err := ioutil.TempFile("", fmt.Sprintf("%s%s", name, tmpSuffix))
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary file")
	}
	if err := tf.Chmod(0666); err != nil {
		return "", errors.Wrap(err, "failed to chmod temporary file")
	}

	return WritableFile(tf.Name()), nil
}

// Move moves a file that chrome binary test writes, to outDir.
func (wf WritableFile) Move(outDir string) error {
	srcFile, err := os.Open(string(wf))
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", wf)
	}
	defer srcFile.Close()

	dstPath, err := getDstPath(outDir, string(wf))
	if err != nil {
		return errors.Wrapf(err, "failed to create destination path")
	}
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s", dstPath)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return errors.Wrapf(err, "failed to copy from %s to %s", string(wf), dstPath)
	}
	return nil
}

// Close removes the file in the temporal directory.
func (wf WritableFile) Close() error {
	return os.Remove(string(wf))
}

// getDstPath returns the file path for the destination file in Move() from outDir and the source file name f.
func getDstPath(outDir, f string) (string, error) {
	name := filepath.Base(f)
	startIndex := strings.Index(name, tmpSuffix)
	if startIndex == -1 {
		return "", errors.Errorf("%s is not found in %s", tmpSuffix, name)
	}
	return filepath.Join(outDir, name[:startIndex]), nil
}
