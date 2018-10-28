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

// CreateWritableFile creates a temporary file that chrome binary test can write to.
func NewWritableFile(name string) (path string, err error) {
	tf, err := ioutil.TempFile("", name+".tast_chrometest.")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary file")
	}
	defer tf.Close()

	if err := tf.Chmod(0666); err != nil {
		return "", errors.Wrap(err, "failed to chmod temporary file")
	}

	return tf.Name(), nil
}

// MoveFile moves a file src that chrome binary test writes, to dst.
func MoveFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return errors.Wrapf(err, "failed to copy from %s to %s", src, dst)
	}
	return os.Remove(src)
}
