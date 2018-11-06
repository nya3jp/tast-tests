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

// CreateWritableTempFile creates a temporary file that Chrome binary tests can write to.
// The name arg should be a short string uniquely identifying the test.
func CreateWritableTempFile(name string) (path string, err error) {
	tf, err := ioutil.TempFile("", name+".tast_chrometest.")
	if err != nil {
		return "", err
	}
	defer tf.Close()

	if err := tf.Chmod(0666); err != nil {
		os.Remove(tf.Name())
		return "", err
	}

	return tf.Name(), nil
}

// CreateWritableTempDir creates a temporary directory in the default directory
// for temporary files on the device (e.g. /tmp). The name arg should be a short
// string uniquely identifying the test.
func CreateWritableTempDir(name string) (path string, err error) {
	dirName, err := ioutil.TempDir("", name+".tast_chrometest.")
	if err != nil {
		return "", err
	}

	if err := os.Chmod(dirName, 0777); err != nil {
		os.RemoveAll(dirName)
		return "", err
	}

	return dirName, nil
}

// CopyFile copies the specified src file to the dst file.
func CopyFile(src, dst string) error {
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
		return err
	}

	return nil
}

// MoveFile moves the specified src file to the dst file.
func MoveFile(src, dst string) error {
	CopyFile(src, dst)
	return os.Remove(src)
}

// RemoveFile removes the file with specified name.
func RemoveFile(name string) error {
	return os.Remove(name)
}

// RemoveDir removes the directory with specified name and all of it's contents.
func RemoveDir(name string) error {
	return os.RemoveAll(name)
}
