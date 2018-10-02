// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chrometest is used to execute compiled Chrome tests.
package chrometest

import (
	"context"
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

// Run executes a chrome binary test, execFileName, with args. This returns error if
// the chrome binary test fails.
func Run(ctx context.Context, outDir, execFileName string, args []string) error {
	const binaryTestDir = "/usr/local/libexec/chrome-binary-tests/"
	binaryTestPath := filepath.Join(binaryTestDir, execFileName)

	// Binary test is executed as chronos.
	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-u", "chronos", binaryTestPath}, args...)...)
	cmd.Env = append(os.Environ(),
		"CHROME_DEVEL_SANDBOX=/opt/google/chrome/chrome-sandbox",
	)

	testing.ContextLogf(ctx, "Executing %s %s", execFileName, testexec.ShellEscapeArray(args))
	if output, err := cmd.CombinedOutput(); err != nil {
		// Write output to a file in outDir.
		fname := "output_" + execFileName + strconv.FormatInt(time.Now().Unix(), 10)
		ioutil.WriteFile(filepath.Join(outDir, fname), output, 0644)

		return errors.Wrapf(err, "%s failed", binaryTestPath)
	}
	return nil
}

// CreateWritableFile creates a file in the sub directory of outDir that chrome binary test can write to.
func CreateWritableFile(outDir, fname string) (string, error) {
	var uid int
	var gid int
	var err error
	if uid, gid, err = getChronosUIDAndGID(); err != nil {
		return "", errors.Wrap(err, "failed to get chronos uid")
	}
	writableSubDir := filepath.Join(outDir, "testout")
	if err = createWritableDirIfNotExist(writableSubDir, uid, gid); err != nil {
		return "", errors.Wrap(err, "failed to create writable subdir")
	}

	fpath := filepath.Join(writableSubDir, fname)
	// Creates a file in outDir.
	var file *os.File
	if file, err = os.OpenFile(fpath, os.O_APPEND|os.O_CREATE, 0644); err != nil {
		return "", err
	}
	defer file.Close()

	// Change the owner of the file to "chronos."
	if err = file.Chown(uid, gid); err != nil {
		return "", err
	}

	return fpath, nil
}

func getChronosUIDAndGID() (int, int, error) {
	chronos, err := user.Lookup("chronos")
	if err != nil {
		return 0, 0, err
	}

	uid, err := strconv.Atoi(chronos.Uid)
	if err != nil {
		return 0, 0, err
	}

	gid, err := strconv.Atoi(chronos.Gid)
	if err != nil {
		return 0, 0, err
	}

	return uid, gid, nil
}

func createWritableDirIfNotExist(writableDir string, uid, gid int) error {
	if _, err := os.Stat(writableDir); os.IsNotExist(err) {
		// This is the first time to create a test writable file.
		// Create a test writable sub directory in outDir.
		if err := os.Mkdir(writableDir, 0644); err != nil {
			return err
		}

		// Change the owner of the directory to "chronos".
		if err = os.Chown(writableDir, uid, gid); err != nil {
			return err
		}
	}
	return nil
}
