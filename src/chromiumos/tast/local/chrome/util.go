// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome/internal/extension"
)

// ChownContentsToChrome recursively changes the ownership of the directory
// contents to the uid and gid of the Chrome's browser process.
func ChownContentsToChrome(dir string) error {
	return extension.ChownContentsToChrome(dir)
}

// moveUserCrashDumps copies the contents of the user crash directory to the
// system crash directory.
func moveUserCrashDumps() error {
	// Normally user crashes are written to /home/user/(hash)/crash as they
	// contain PII, but for test images they are written to /home/chronos/crash.
	// https://crrev.com/c/1986701
	const (
		userCrashDir   = "/home/chronos/crash"
		systemCrashDir = "/var/spool/crash"
		crashGroup     = "crash-access"
	)

	g, err := user.LookupGroup(crashGroup)
	if err != nil {
		return err
	}
	gid, err := strconv.ParseInt(g.Gid, 10, 32)
	if err != nil {
		return errors.Wrapf(err, "failed to parse gid %q", g.Gid)
	}

	if err := os.MkdirAll(systemCrashDir, 02770); err != nil {
		return err
	}

	if err := os.Chown(systemCrashDir, 0, int(gid)); err != nil {
		return err
	}

	fis, err := ioutil.ReadDir(userCrashDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, fi := range fis {
		if !fi.Mode().IsRegular() {
			continue
		}

		// As they are in different partitions, os.Rename() doesn't work.
		src := filepath.Join(userCrashDir, fi.Name())
		dst := filepath.Join(systemCrashDir, fi.Name())
		if err := fsutil.MoveFile(src, dst); err != nil {
			return err
		}

		if err := os.Chown(dst, 0, int(gid)); err != nil {
			return err
		}
	}

	return nil
}
