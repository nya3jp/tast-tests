// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"chromiumos/tast/errors"
)

const chromeUser = "chronos" // Chrome Unix username

// ChownContentsToChrome recursively changes the ownership of the directory
// contents to the uid and gid of the Chrome's browser process.
func ChownContentsToChrome(dir string) error {
	return chownContents(dir, chromeUser)
}

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
