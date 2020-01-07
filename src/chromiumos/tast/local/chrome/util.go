// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
)

// chownContents recursively chowns dir's contents to username's uid and gid.
func chownContents(dir string, username string) error {
	var u *user.User
	var err error
	if u, err = user.Lookup(username); err != nil {
		return err
	}

	var uid, gid int64
	if uid, err = strconv.ParseInt(u.Uid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse uid %q", u.Uid)
	}
	if gid, err = strconv.ParseInt(u.Gid, 10, 32); err != nil {
		return errors.Wrapf(err, "failed to parse gid %q", u.Gid)
	}

	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, int(uid), int(gid))
	})
}

// moveUserCrashDumps copies the contents of the user crash directory to the
// system crash directory.
func moveUserCrashDumps() error {
	const (
		userCrashDir   = "/home/chronos/crash"
		systemCrashDir = "/var/spool/crash"
	)

	if err := os.MkdirAll(systemCrashDir, 02770); err != nil {
		return err
	}

	fis, err := ioutil.ReadDir(userCrashDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	copyFile := func(dst, src string) error {
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()

		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()

		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	}

	for _, fi := range fis {
		if !fi.Mode().IsRegular() {
			continue
		}

		// As they are in different partitions, os.Rename() doesn't work.
		dst := filepath.Join(systemCrashDir, fi.Name())
		src := filepath.Join(userCrashDir, fi.Name())
		if err := copyFile(dst, src); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil {
			return err
		}
	}

	return nil
}
